// Same idea as localhost-proxy.mjs, for the Tourist App e2e (make app-e2e): inside the Playwright
// container, http://localhost:3002 / :8000 / :8090 reach app / gateway / mockext like on the host,
// because the Flutter build calls the gateway at http://localhost:8000 from the browser.
import net from 'node:net';

const routes = { 3002: ['app', 80], 8000: ['gateway', 8080], 8090: ['mockext', 8090] };

for (const [port, [host, target]] of Object.entries(routes)) {
  net
    .createServer((c) => {
      const u = net.connect(target, host);
      c.pipe(u).pipe(c);
      const close = () => { c.destroy(); u.destroy(); };
      c.on('error', close);
      u.on('error', close);
    })
    .listen(Number(port), () => console.log(`localhost:${port} -> ${host}:${target}`));
}
