import ws from 'k6/ws';
import { check } from 'k6';

import { randomIntBetween } from 'https://jslib.k6.io/k6-utils/1.2.0/index.js';

export default function () {
  const url = 'ws://localhost:8080';

  const res = ws.connect(url, {}, function (socket) {
    let instrs = [1, 2, 3, 4, 5]

    socket.on("open", () => console.log("connected"))

    socket.on("close", () => console.log("closed"))

    socket.on("error", (e) => console.log("error", e))

    socket.setInterval(() => {
      socket.send(JSON.stringify({ 'm': 1, 'i': instrs }))
      instrs = [
        randomIntBetween(1, 100),
        randomIntBetween(1, 100),
        randomIntBetween(1, 100),
        randomIntBetween(1, 100),
        randomIntBetween(1, 100),
      ]
    }, 1000)

    socket.setTimeout(() => {
      socket.close()
    }, 5000)
  });

  if(res.status !== 101) console.log(res.error);

  check(res, { 'status is 101': (r) => r && r.status === 101 });
}
