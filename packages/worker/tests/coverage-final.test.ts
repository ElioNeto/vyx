import { vi, describe, it, expect, beforeEach, afterEach } from 'vitest';
import net from 'net';
import { setupProcessHandlers } from '../src/dispatch';

describe('Process Handlers Coverage', () => {
  let mockExit: any;
  let mockConsoleLog: any;
  
  beforeEach(() => {
    mockExit = vi.spyOn(process, 'exit').mockImplementation((() => undefined) as any);
    mockConsoleLog = vi.spyOn(console, 'log').mockImplementation(() => {});
  });
  
  afterEach(() => {
    vi.restoreAllMocks();
  });
  
  it('should log and exit on SIGTERM when shouldExit is true', () => {
    const socket = new net.Socket();
    setupProcessHandlers(socket, true);
    
    process.emit('SIGTERM');
    expect(mockConsoleLog).toHaveBeenCalled();
    expect(mockExit).toHaveBeenCalledWith(0);
  });
  
  it('should log and exit on SIGINT when shouldExit is true', () => {
    const socket = new net.Socket();
    setupProcessHandlers(socket, true);
    
    process.emit('SIGINT');
    expect(mockConsoleLog).toHaveBeenCalled();
    expect(mockExit).toHaveBeenCalledWith(0);
  });
});
