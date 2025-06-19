import { describe, it, expect } from '@jest/globals';

describe('Basic Framework Test', () => {
  it('should be able to run tests', () => {
    expect(1 + 1).toBe(2);
  });

  it('should validate test infrastructure', () => {
    const mockFunction = jest.fn();
    mockFunction('test');
    expect(mockFunction).toHaveBeenCalledWith('test');
  });

  it('should handle async operations', async () => {
    const asyncFunction = async () => {
      return Promise.resolve('success');
    };

    const result = await asyncFunction();
    expect(result).toBe('success');
  });
});