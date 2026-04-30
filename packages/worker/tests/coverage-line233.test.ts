import { createAndSetupSocket } from '../src/dispatch';
import net from 'net';

describe('Cover line 233 test', () => {
  it('should trigger data handler in createAndSetupSocket', (done) => {
    // Create a TCP server
    const server = net.createServer((serverSocket) => {
      // When client connects, send a frame with data
      const payload = JSON.stringify({ method: 'GET', path: '/test' });
      const payloadBuf = Buffer.from(payload);
      const frame = Buffer.alloc(5 + payloadBuf.length);
      frame.writeUInt32LE(payloadBuf.length, 0);
      frame.writeUInt8(0x01, 4); // TYPE_REQUEST
      payloadBuf.copy(frame, 5);
      
      serverSocket.write(frame);
      
      // Close server socket after sending
      setTimeout(() => {
        serverSocket.end();
      }, 100);
    });

    server.listen(0, () => { // Listen on random port
      const addr = server.address() as net.AddressInfo;
      const port = addr.port;
      
      // Connect to the TCP server
      const socket = createAndSetupSocket(
        `localhost:${port}`,
        'test-worker-233',
        [],
        false
      );

      socket.on('close', () => {
        server.close();
        done();
      });

      // Timeout safeguard
      setTimeout(() => {
        socket.destroy();
        server.close();
        done();
      }, 1000);
    });
  }, 3000);
});
