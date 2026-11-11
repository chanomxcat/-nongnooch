// Makes http://localhost:{3000,3001,8090} inside the Playwright container behave like on the host
// (web, admin, mockext), so the in-container browser follows the same URLs a presenter's browser does
// (e.g. mockext paymentUrl = http://localhost:8090/...). Started by playwright.config webServer;
// on the host the ports are already taken, so Playwright reuses them and this never runs.
import net from 'node:net';

const routes = { 3000: ['web', 3000], 3001: ['admin', 3000], 8090: ['mockext', 8090] };

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
