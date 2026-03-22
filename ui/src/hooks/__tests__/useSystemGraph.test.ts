import { describe, it, expect } from 'vitest';

describe('useSystemGraph', () => {
  it('should be importable', async () => {
    const mod = await import('../useSystemGraph');
    expect(mod.useSystemGraph).toBeTypeOf('function');
  });
});
