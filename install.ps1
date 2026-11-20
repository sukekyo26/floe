# install.ps1 - fetch a floe release for Windows and put it where you can run it.
#
# Usage:
#   irm https://raw.githubusercontent.com/sukekyo26/floe/main/install.ps1 | iex
#
# Environment overrides:
#   FLOE_VERSION      Version to install (e.g. "0.2.0"); default "latest".
#   FLOE_INSTALL_DIR  Where to put floe.exe; default "%USERPROFILE%\.local\bin".
#   FLOE_ADD_TO_PATH  "1" adds that folder to your user PATH; default leaves PATH alone.
#   FLOE_INSTALL_TIMEOUT  Seconds to wait for an answer while nothing comes, for each request; default "120".
#
# The binary's SHA-256 is verified against SHA256SUMS from the same release.
# Runs on Windows PowerShell 5.1 and PowerShell 7. This file is ASCII with no BOM: `irm | iex` fails on a BOM.

# A block of its own: what it sets ends with it, as `irm | iex` runs this in the caller's session.
& {
  $ErrorActionPreference = 'Stop'
  # Windows PowerShell draws the bar for every chunk: 32 MiB took 30 s with it and 0.1 s without.
  $ProgressPreference = 'SilentlyContinue'
  # A caller in strict mode would make reading a property that is not there an error in here too.
  Set-StrictMode -Off

  $repo = 'sukekyo26/floe'
  # Run from a file, the block holds the file's path; from `irm | iex`, none.
  $self = $MyInvocation.MyCommand.ScriptBlock.File
  # LIMIT: an alias the caller made with the name of a function here (Fetch, Place, Install, ...) is found before the
  # function. No alias PowerShell makes has one of these names.

  # Windows PowerShell answers with a WebHeaderCollection, read by its index; PowerShell 7 with
  # HttpResponseHeaders, which has no index but a Location. That is a Uri, read as it was written: its ToString()
  # unescapes, and a lower-case %7c comes back as %7C, which a signed URL does not accept.
  function LocationOf($headers) {
    if ($headers -is [System.Net.WebHeaderCollection]) { return $headers['Location'] }
    if ($headers -is [System.Collections.IDictionary]) { return @($headers['Location'])[0] }
    $to = $headers.Location
    if ($to -is [uri]) { return $to.OriginalString }
    return $to
  }

  # PowerShell 7.4 and later refuse a redirect from https to http even when told to follow none, saying only that
  # the operation is not valid: with this they hand it back, and Fetch refuses it by name. Nothing is followed.
  $insecure = @{}
  if ((Get-Command Microsoft.PowerShell.Utility\Invoke-WebRequest).Parameters.ContainsKey('AllowInsecureRedirect')) {
    $insecure['AllowInsecureRedirect'] = $true
  }
  # PowerShell 7.4 and later can give up on a body that stops coming; before, nothing says so.
  $stalls = (Get-Command Microsoft.PowerShell.Utility\Invoke-WebRequest).Parameters.ContainsKey('OperationTimeoutSeconds')

  # Every hop is asked for with no redirect followed, and only an https one is taken: left to itself
  # Windows PowerShell follows https -> http -> https without a word. A failure with an answer carries its status.
  # LIMIT: TLS is not set: a Windows PowerShell on a .NET Framework whose default is not TLS 1.2 fails with "Could not
  # create SSL/TLS secure channel". Only the runners' Windows Server 2022 and 2025 were tried.
  # A server or a proxy that goes silent is given up on after $wait seconds (FLOE_INSTALL_TIMEOUT), and told as that
  # where PowerShell tells it apart: Windows PowerShell by the status of its WebException, PowerShell 7.4 and later
  # by the id of the error (seen on 5.1 and 7.6, run 44263536485; 7.4's source makes the same ids).
  # LIMIT: -TimeoutSec is the wait for an answer to begin (the proxy, TLS, the headers), not for the body. A body
  # that stops is given up on after $wait seconds by PowerShell 7.4 and later; after .NET's own 5 minutes by Windows
  # PowerShell, whatever $wait is; never by PowerShell 7.0 to 7.3, which also get no word of a time-out added.
  # LIMIT: the time of the whole download has no limit, which a slow line would run into: a server that keeps
  # sending, however little, sooner than the limit each time is waited for to the end. Each request has the limit
  # anew, a redirect's too: five or so that each go silent just short of it take ten minutes.
  # LIMIT: what Windows PowerShell does before a request turns asynchronous (the name, the proxy, the connection:
  # .NET's BeginGetResponse) may not be under -TimeoutSec. A stop of the look of run 44244107780's (nothing sent
  # after the proxy's CONNECT) ended once as a silence under it (run 44694367866); that every such stop does is not
  # known. One seen to run past FLOE_INSTALL_TIMEOUT would call for a limit on the whole of a request.
  function Fetch([string]$Url, [string]$OutFile) {
    $limit = @{ TimeoutSec = $wait }
    if ($stalls) { $limit['OperationTimeoutSeconds'] = $wait }
    $at = [uri]$Url
    for ($hop = 0; $hop -le 10; $hop++) {
      if ($at.Scheme -cne 'https') { throw "download failed: $Url (redirected to $($at.Scheme))" }
      $status = 0; $to = $null; $problem = $null; $got = $null
      try {
        # Windows PowerShell hands a redirect back with an error that does not stop it;
        # PowerShell 7 throws it, the response inside.
        # No -OutFile: Windows PowerShell reads it as a wildcard, and a [ or ] in TEMP is not found, escaped or not.
        $got = Invoke-WebRequest -UseBasicParsing -Uri $at -MaximumRedirection 0 `
          -ErrorAction SilentlyContinue -ErrorVariable problem @insecure @limit
        if ($null -eq $got) { throw $problem[0] }
        $status = [int]$got.StatusCode
        $to = LocationOf $got.Headers
      } catch {
        $answer = $_.Exception.Response
        if (-not $answer) {
          $failed = "download failed: $Url ($($_.Exception.Message))"
          $gone = $_.Exception -is [Net.WebException] -and $_.Exception.Status -eq [Net.WebExceptionStatus]::Timeout
          if ($gone -or $_.FullyQualifiedErrorId -clike 'ConnectionTimeoutReached,*' -or $_.FullyQualifiedErrorId -clike 'OperationTimeoutReached,*') {
            $failed += "; check your network and proxy and run it again; FLOE_INSTALL_TIMEOUT sets how many seconds to wait for an answer (now $wait)"
          }
          throw $failed
        }
        $status = [int]$answer.StatusCode
        $to = LocationOf $answer.Headers
      }
      if ($status -eq 200) {
        [IO.File]::WriteAllBytes($OutFile, $got.RawContentStream.ToArray())
        return
      }
      if ($status -lt 300 -or $status -ge 400) {
        $failed = New-Object Exception "download failed: $Url (status $status)"
        $failed.Data['status'] = $status
        throw $failed
      }
      if (-not $to) { throw "download failed: $Url (redirected to nowhere)" }
      $at = New-Object System.Uri($at, [string]$to)
    }
    throw "download failed: $Url (too many redirects)"
  }

  # A folder as one PATH entry: no \ at the end, but a drive's root keeps it (D: is that drive's current folder).
  function Entry([string]$path) {
    try { $root = [IO.Path]::GetPathRoot($path) } catch { $root = $null }
    if ($root -and $path.Equals($root, [StringComparison]::OrdinalIgnoreCase)) { return $path }
    return $path.TrimEnd('\')
  }

  # Whether dir is an entry of path. Windows opens %VAR% in the user's Path only when it is an ExpandString,
  # and never in a process's PATH: with expand, an entry is also read opened.
  function OnPath([string]$dir, $path, [bool]$expand) {
    if ($path -isnot [string]) { return $false }
    $want = Entry $dir
    foreach ($item in $path.Split(';')) {
      $e = $item.Trim('"')
      $forms = @($e)
      if ($expand) { $forms += [Environment]::ExpandEnvironmentVariables($e) }
      foreach ($form in $forms) {
        if ($form -and (Entry $form).Equals($want, [StringComparison]::OrdinalIgnoreCase)) { return $true }
      }
    }
    return $false
  }

  # A value as a PowerShell string in single quotes. PowerShell's own escaping: it also reads the curly
  # quotes U+2018 to U+201B as quotes, which doubling ' alone would leave open.
  function Quote([string]$value) {
    return "'" + [Management.Automation.Language.CodeGeneration]::EscapeSingleQuotedStringContent($value) + "'"
  }

  # The user's Path as written (%VAR% left closed) and its kind; $null and ExpandString when there is none.
  function UserPath {
    $key = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey('Environment')
    if (-not $key) { return @($null, [Microsoft.Win32.RegistryValueKind]::ExpandString) }
    try {
      $value = $key.GetValue('Path', $null, [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames)
      if ($null -eq $value) { return @($null, [Microsoft.Win32.RegistryValueKind]::ExpandString) }
      return @($value, $key.GetValueKind('Path'))
    } finally { $key.Close() }
  }

  # Adds dir at the end of the user's Path, leaving every other entry and the kind of the value as they are:
  # setting it through [Environment] would write %JAVA_HOME%\bin opened, as it is now, and as a String.
  function AddToUserPath([string]$dir) {
    $key = [Microsoft.Win32.Registry]::CurrentUser.CreateSubKey('Environment')
    try {
      $raw = $key.GetValue('Path', $null, [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames)
      $kind = [Microsoft.Win32.RegistryValueKind]::ExpandString
      if ($null -eq $raw) { $raw = '' } else { $kind = $key.GetValueKind('Path') }
      if ($raw -isnot [string]) { throw "the user's Path is not a string ($kind)" }
      $new = $raw.TrimEnd(';')
      if ($new) { $new += ';' }
      $key.SetValue('Path', $new + (Entry $dir), $kind)
    } finally { $key.Close() }
    # LIMIT: a roundabout way to tell running programs that the environment changed: removing a variable that is not
    # there makes .NET broadcast WM_SETTINGCHANGE and changes nothing. Telling it directly needs SendMessageTimeout,
    # so C# compiled here by Add-Type: slow, and what scanners look at.
    $notice = 'FLOE_INSTALL_PATH_NOTICE'
    if ($null -eq [Environment]::GetEnvironmentVariable($notice, 'User')) {
      [Environment]::SetEnvironmentVariable($notice, $null, 'User')
    }
  }

  # What to say of PATH once floe.exe is in place: the user's Path decides whether to write it, the process's
  # whether anything is said.
  function PathNote([string]$dir, [string]$exe, [string]$version) {
    $add = $env:FLOE_ADD_TO_PATH -ceq '1'
    $user = UserPath
    $onUser = OnPath $dir $user[0] ($user[1] -eq [Microsoft.Win32.RegistryValueKind]::ExpandString)
    $onProcess = OnPath $dir $env:Path $false
    $fails = "installed floe $version to $exe, but could not add $dir to your PATH"
    if ($onUser) {
      if (-not $onProcess) { Write-Host "floe-install: $dir is on your PATH; open a new terminal to use it" }
      return
    }
    $oneEntry = -not ($dir.Contains(';') -or $dir.Contains('%'))
    if ($add) {
      if ($null -ne $user[0] -and $user[0] -isnot [string]) { throw "${fails}: the user's Path is not a string ($($user[1]))" }
      if (-not $oneEntry) { throw "${fails}: it cannot be written as one PATH entry" }
      try { AddToUserPath $dir } catch { throw "${fails}: $($_.Exception.Message)" }
      Write-Host "floe-install: added $dir to your PATH; open a new terminal to use it (sign in again if floe is still not found)"
      return
    }
    if ($onProcess) { return }
    if (-not $oneEntry) {
      Write-Host "floe-install: $dir is not on your PATH; it cannot be written as one PATH entry"
      return
    }
    $line = ''
    if ($env:FLOE_INSTALL_DIR) { $line += '$env:FLOE_INSTALL_DIR = ' + (Quote $dir) + '; ' }
    if ($env:FLOE_VERSION) { $line += '$env:FLOE_VERSION = ' + (Quote $env:FLOE_VERSION) + '; ' }
    if ($env:FLOE_INSTALL_TIMEOUT) { $line += '$env:FLOE_INSTALL_TIMEOUT = ' + (Quote $env:FLOE_INSTALL_TIMEOUT) + '; ' }
    $line += '$env:FLOE_ADD_TO_PATH = ''1''; '
    if ($self) { $line += '& ' + (Quote $self) } else { $line += "irm https://raw.githubusercontent.com/$repo/main/install.ps1 | iex" }
    Write-Host "floe-install: $dir is not on your PATH. To add it, run:"
    Write-Host "  $line"
    Write-Host 'floe-install: then open a new terminal'
  }

  # Puts the verified file at dir\floe.exe. A running floe.exe can be renamed but not overwritten nor removed, so
  # it is moved aside first, under a name of its own: an older one may still run under the last name used.
  # LIMIT: a scanner looking at the new file may refuse the renames for a moment; there is no retry, and a refused
  # rename leaves floe.exe as it was. Two installs at once may remove the one moved aside by the other, which then
  # has nothing to put back if its last rename fails.
  function Place([string]$file, [string]$dir) {
    $cannotWrite = "cannot write to $dir; set FLOE_INSTALL_DIR to a directory you can write to"
    try { [IO.Directory]::CreateDirectory($dir) | Out-Null } catch { throw $cannotWrite }
    $exe = [IO.Path]::Combine($dir, 'floe.exe')
    $tag = [IO.Path]::GetRandomFileName()
    $new = "$exe.new-$tag"
    $old = "$exe.old-$tag"
    $placed = $false
    $moved = $false
    try {
      # In the try: a copy cut short, or stopped by Ctrl+C, leaves no .new- behind.
      try { Copy-Item -LiteralPath $file -Destination $new } catch { throw $cannotWrite }
      try {
        if (Test-Path -LiteralPath $exe) {
          Move-Item -LiteralPath $exe -Destination $old
          $moved = $true
        }
        Move-Item -LiteralPath $new -Destination $exe
        $placed = $true
      } catch {
        throw "cannot replace ${exe}: $($_.Exception.Message); close what has it open, or stop floe (floe stop), and try again"
      }
    } finally {
      # Also on Ctrl+C. What fails here is let go: the error that brought us here is the one to tell.
      if (-not $placed) {
        if ($moved -and -not (Test-Path -LiteralPath $exe)) {
          Move-Item -LiteralPath $old -Destination $exe -ErrorAction SilentlyContinue
        }
        Remove-Item -LiteralPath $new -Force -ErrorAction SilentlyContinue
      }
    }
    # LIMIT: one still running stays until a later install; one left by a killed install (with its .new-) stays too.
    Get-ChildItem -LiteralPath $dir -Filter 'floe.exe.old-*' -File | ForEach-Object {
      Remove-Item -LiteralPath $_.FullName -Force -ErrorAction SilentlyContinue
    }
    return $exe
  }

  function Install {
    if ([Environment]::OSVersion.Platform -ne [PlatformID]::Win32NT) {
      throw 'install.ps1 installs floe on Windows; on Linux and macOS run install.sh: curl -fsSL https://raw.githubusercontent.com/sukekyo26/floe/main/install.sh | sh'
    }

    # A 32-bit PowerShell sees x86, and the machine's own in PROCESSOR_ARCHITEW6432.
    $machine = $env:PROCESSOR_ARCHITEW6432
    if (-not $machine) { $machine = $env:PROCESSOR_ARCHITECTURE }
    switch -CaseSensitive ($machine) {
      'AMD64' { $arch = 'amd64' }
      'ARM64' { $arch = 'arm64' }
      default { throw "unsupported architecture: $machine (floe supports amd64 and arm64)" }
    }
    # An x64 PowerShell on an arm64 machine sees AMD64 in both: the runtime knows better, where it is new enough.
    # LIMIT: amd64 runners never take this; it was seen once on windows-11-arm (run 43840628538).
    if ($arch -ceq 'amd64') {
      # .NET Framework before 4.7.1 has no RuntimeInformation.
      $info = 'Runtime.InteropServices.RuntimeInformation' -as [type]
      if ($info -and $info::OSArchitecture -eq 'Arm64') { $arch = 'arm64' }
    }

    $add = $env:FLOE_ADD_TO_PATH
    if ($add -and $add -cne '1') { throw "FLOE_ADD_TO_PATH must be 1 or unset (got $add)" }

    # Read as letters, as install.sh reads it: digits alone, the first not 0, five at most (PowerShell 7 refuses
    # more than 2147483 seconds). \A and \z: ^ and $ would take a line break at the end.
    $wait = $env:FLOE_INSTALL_TIMEOUT
    if (-not $wait) { $wait = '120' }
    if (-not ($wait -cmatch '\A[1-9][0-9]{0,4}\z')) {
      throw "FLOE_INSTALL_TIMEOUT must be a number of seconds from 1 to 99999, in digits alone (got $wait)"
    }
    $wait = [int]$wait

    $given = $env:FLOE_INSTALL_DIR
    if (-not $given) {
      if (-not $env:USERPROFILE) { throw 'USERPROFILE is not set; set FLOE_INSTALL_DIR to a directory you can write to' }
      $given = [IO.Path]::Combine($env:USERPROFILE, '.local', 'bin')
    }
    # One absolute path, from where you are now: .NET would read a relative one from where PowerShell started.
    $provider = $null
    $drive = $null
    $dir = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($given, [ref]$provider, [ref]$drive)
    # One sentence for FLOE_INSTALL_DIR and for the default from USERPROFILE alike.
    $notLocal = "the install folder must be a folder on a local drive (got $given); set FLOE_INSTALL_DIR to a local folder"
    if ($provider.Name -cne 'FileSystem') { throw $notLocal }
    # The provider keeps / as written (//server/share too); Windows reads it as \.
    $dir = $dir.Replace('/', '\')
    # floe itself refuses UNC paths; \\?\ and \\.\ are in the same form.
    if ($dir.StartsWith('\\', [StringComparison]::Ordinal)) { throw $notLocal }

    $version = $env:FLOE_VERSION
    $tag = $null
    if ($version -and $version -cne 'latest') {
      if ($version.StartsWith('v', [StringComparison]::Ordinal)) { $tag = $version } else { $tag = "v$version" }
      CheckTag $tag
    }

    $asset = "floe-windows-$arch.exe"
    $tmp = [IO.Path]::Combine([IO.Path]::GetTempPath(), 'floe-install-' + [IO.Path]::GetRandomFileName())
    [IO.Directory]::CreateDirectory($tmp) | Out-Null
    try {
      if (-not $tag) {
        $api = "https://api.github.com/repos/$repo/releases/latest"
        $json = [IO.Path]::Combine($tmp, 'release.json')
        try { Fetch $api $json } catch {
          if (-not $_.Exception.Data['status']) { throw }
          throw "$($_.Exception.Message); no release yet, or GitHub's API rate limit: try again later, or set FLOE_VERSION to a version, which skips the API"
        }
        try { $tag = ([IO.File]::ReadAllText($json) | ConvertFrom-Json).tag_name } catch { $tag = $null }
        if ($tag -isnot [string] -or -not $tag) { throw "could not parse tag_name from $api" }
        CheckTag $tag
      }
      $shown = $tag.Substring(1)
      $base = "https://github.com/$repo/releases/download/$tag"
      Write-Host "floe-install: downloading $asset@$shown"
      $file = [IO.Path]::Combine($tmp, $asset)
      $sums = [IO.Path]::Combine($tmp, 'SHA256SUMS')
      # Whatever went wrong, one plain hint: an old or mistyped version has no Windows build.
      # LIMIT: Windows PowerShell with -MaximumRedirection 0 may say github.com's 404 is a closed connection, not a
      # status (run 43862510949; one of two runs on a real machine); the hint is all that tells why.
      try { Fetch "$base/$asset" $file } catch {
        throw "$($_.Exception.Message); check that floe $shown has a Windows build: https://github.com/$repo/releases"
      }
      Fetch "$base/SHA256SUMS" $sums

      # Our own release writes "<lowercase sha256>  <name>" lines, LF: that whole line, letter for letter, and
      # only once. A BOM is kept, not dropped; -eq and -ceq would let one through.
      $text = (New-Object System.Text.UTF8Encoding $false).GetString([IO.File]::ReadAllBytes($sums))
      $found = @($text.Split("`n") | Where-Object { $_.EndsWith("  $asset", [StringComparison]::Ordinal) })
      if ($found.Count -eq 0) { throw "$asset not listed in SHA256SUMS" }
      $actual = (Get-FileHash -Algorithm SHA256 -LiteralPath $file).Hash.ToLowerInvariant()
      if ($found.Count -ne 1 -or -not $found[0].Equals("$actual  $asset", [StringComparison]::Ordinal)) {
        throw "checksum mismatch ($actual != $(($found | ForEach-Object { ($_ -split ' ')[0] }) -join ', '))"
      }

      $exe = Place $file $dir
    } finally {
      # A file a scanner still holds must not turn a finished install into a failure, nor hide the error that
      # brought us here: the folder is in the temp folder, which the system clears.
      Remove-Item -Recurse -Force -LiteralPath $tmp -ErrorAction SilentlyContinue
    }
    Write-Host "floe-install: installed floe $shown to $exe"
    PathNote $dir $exe $shown
    Write-Host 'floe-install: for your agent: run floe skill install <agent> (floe skill install --help lists them)'
  }

  # The version is a part of a URL path: letters, digits, . + - only, and no "..", which the client would fold.
  function CheckTag([string]$tag) {
    if (-not ($tag -cmatch '\Av[0-9A-Za-z.+-]+\z') -or $tag.Contains('..')) {
      throw "unsupported version: $tag; set FLOE_VERSION to a version like 0.2.0, or unset it for the latest"
    }
  }

  # Not exit: under `irm | iex` it would end the caller's PowerShell. And every error leaves as a throw:
  # an error that is not one ends `powershell -Command "irm ... | iex"` with exit code 0.
  try { Install } catch { throw ('floe-install: ' + $_.Exception.Message) }
}
