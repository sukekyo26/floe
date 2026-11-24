import { setBoot, type Boot } from '@/config/boot';

// testBoot sets the boot values the components' tests draw with: each message as its key and its args
export function testBoot(): void {
  const messages = new Proxy({} as Boot['messages'], { get: (_, key) => `${String(key)}:%s` });
  setBoot({
    screen: 'list',
    lang: 'en',
    messages,
    framed: '',
    served: false,
    version: '',
    states: [],
    maxURLs: 10,
  });
}
