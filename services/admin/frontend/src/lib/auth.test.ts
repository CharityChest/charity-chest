import { describe, it, expect, beforeEach } from 'vitest';
import { getToken, setToken, clearToken, isAuthenticated, getRole } from './auth';

// Helpers to build a minimal JWT with a given payload.
function makeJwt(payload: Record<string, unknown>): string {
  const header = btoa(JSON.stringify({ alg: 'HS256', typ: 'JWT' }));
  const body = btoa(JSON.stringify(payload));
  return `${header}.${body}.fakesig`;
}

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

describe('getRole', () => {
  it('returns null when no token is stored', () => {
    expect(getRole()).toBeNull();
  });

  it('returns null when the token payload has no role field', () => {
    setToken(makeJwt({ user_id: 1, email: 'a@b.com' }));
    expect(getRole()).toBeNull();
  });

  it('returns "root" when the payload contains role: "root"', () => {
    setToken(makeJwt({ user_id: 1, email: 'a@b.com', role: 'root' }));
    expect(getRole()).toBe('root');
  });

  it('returns "system" when the payload contains role: "system"', () => {
    setToken(makeJwt({ user_id: 1, email: 'a@b.com', role: 'system' }));
    expect(getRole()).toBe('system');
  });

  it('returns null for a malformed token (not three dot-separated parts)', () => {
    setToken('not.a.valid.jwt.at.all');
    // atob on an invalid base64 segment throws — getRole must catch and return null
    expect(getRole()).toBeNull();
  });

  it('returns null when role is not a string (e.g. a number)', () => {
    setToken(makeJwt({ user_id: 1, role: 42 }));
    expect(getRole()).toBeNull();
  });
});
