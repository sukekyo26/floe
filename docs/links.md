# Linked files and folders

`floe publish --link` registers an HTML file, a data file or a folder where it is, instead of storing a page. [commands.md](commands.md#linked-files) tells what can be linked, what is refused and the slug it takes.

## An HTML file

An HTML file is shown as it is: floe adds nothing around it, so a page of your own or one another tool made shows as written. The files under its directory are served too, for its relative references (CSS, scripts, images). Saving the file reloads the open tab.

## A data file

A linked data file is drawn each time its page is opened, with `--theme` as given when it was linked, so that a page written in a repository can be read while it is written:

- Saving the data file, a part, theme or template of your own, or fetching icons, reloads the open tab. Saving an image it shows does not: save the data file again, or reload the tab.
- When it has mistakes, the tab shows them instead of the page, and comes back to the page once they are fixed and saved.
- Nothing of its directory is served: its images are in its page. Its download is the page drawn, its CSP kept in a `<meta>`.
- Its title in the list is its `title:`, read each time.

## A folder

A linked folder is one row of the management page's Folders tab. Its URL (`/a/<slug>/`) opens that tab with the folder open:

- Opening the row shows what is directly in the folder: its directories, then its HTML files (`.html`, `.htm`), by name. A directory opens the same way, and only what is open is read, so a folder of thousands of files costs only the directories opened. What is open is kept in the browser across a reload, not in the URL.
- Names starting with a dot (`.git`, `.drafts`) are left out, and so are symlinks that lead out of the folder and anything but regular files and directories. A symlink to a directory is not opened. The files under the folder are served as a linked HTML file's are, by the same rules.
- One directory shows at most 1000 entries, the first by name; the row under them says how many there are.
- A page of the folder opens in the frame (`/a/<slug>/?file=<path>`) under the folder's title, with the folder's tree on its left, open where the page is. Drag the tree's edge, or use ← → Home End on it, to widen or narrow it (160 px up to 480 px or half the window), and hide it with the button at its head; the browser keeps its width and whether it shows. With the tree hidden, or on a narrow screen (where it opens over the page), the bar picks among the pages of the page's directory. A page's title is its file's name, and nothing of it is read to list it.
- What is added, removed or renamed in the folder shows when the list is next read. The folder has the row's favourite, archive, tags and title; its pages have none of their own.
- A folder in a repository is titled by where it is in it (`docs`, `docs/v1`), and a folder outside one by its name (`handbook`).
