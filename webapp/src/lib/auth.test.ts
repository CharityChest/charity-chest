import { describe, it, expect, beforeEach } from 'vitest';
import { getToken, setToken, clearToken, isAuthenticated } from './auth';

// localStorage is provided by jsdom; reset before each test for isolation.
beforeEach(() => {
  localStorage.clear();
});

describe('getToken', () => {
  it('returns null when no token is stored', () => {
    expect(getToken()).toBeNull();
  });

  it('returns the stored token', () => {
    localStorage.setItem('cc_token', 'my-jwt');
    expect(getToken()).toBe('my-jwt');
  });
});

describe('setToken', () => {
  it('persists the token to localStorage', () => {
    setToken('abc123');
    expect(localStorage.getItem('cc_token')).toBe('abc123');
  });

  it('overwrites a previously stored token', () => {
    setToken('first');
    setToken('second');
    expect(getToken()).toBe('second');
  });
});

describe('clearToken', () => {
  it('removes the token from localStorage', () => {
    setToken('to-be-removed');
    clearToken();
    expect(getToken()).toBeNull();
  });

  it('is a no-op when no token exists', () => {
    expect(() => clearToken()).not.toThrow();
    expect(getToken()).toBeNull();
  });
});

describe('isAuthenticated', () => {
  it('returns false when no token is stored', () => {
    expect(isAuthenticated()).toBe(false);
  });

  it('returns true after setToken', () => {
    setToken('valid-token');
    expect(isAuthenticated()).toBe(true);
  });

  it('returns false after clearToken', () => {
    setToken('valid-token');
    clearToken();
    expect(isAuthenticated()).toBe(false);
  });
});
