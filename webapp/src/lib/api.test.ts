import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { ApiError, getLocale, api } from './api';

// --- ApiError ---

describe('ApiError', () => {
  it('has name "ApiError"', () => {
    const err = new ApiError(400, 'bad request');
    expect(err.name).toBe('ApiError');
  });

  it('carries the HTTP status code', () => {
    const err = new ApiError(401, 'unauthorized');
    expect(err.status).toBe(401);
  });

  it('carries the message', () => {
    const err = new ApiError(404, 'not found');
    expect(err.message).toBe('not found');
  });

  it('is an instance of Error', () => {
    expect(new ApiError(500, 'oops')).toBeInstanceOf(Error);
  });
});

// --- getLocale ---

describe('getLocale', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('returns "en" for an /en/ path prefix', () => {
    vi.stubGlobal('location', { pathname: '/en/login' });
    expect(getLocale()).toBe('en');
  });

  it('returns "it" for an /it/ path prefix', () => {
    vi.stubGlobal('location', { pathname: '/it/dashboard' });
    expect(getLocale()).toBe('it');
  });

  it('returns "en" for an unknown path prefix', () => {
    vi.stubGlobal('location', { pathname: '/fr/page' });
    expect(getLocale()).toBe('en');
  });

  it('returns "en" for the root path', () => {
    vi.stubGlobal('location', { pathname: '/' });
    expect(getLocale()).toBe('en');
  });

  it('returns "en" when window is undefined (SSR)', () => {
    vi.stubGlobal('window', undefined);
    expect(getLocale()).toBe('en');
  });
});

// --- request (via api methods) ---

describe('api — Accept-Language header', () => {
  beforeEach(() => {
    vi.stubGlobal('location', { pathname: '/it/login' });
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve({
            token: 'tok',
            user: { id: 1, email: 'a@b.com', name: 'A', created_at: '', updated_at: '' },
          }),
      }),
    );
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('sends Accept-Language: it when the locale is Italian', async () => {
    await api.login('a@b.com', 'pass');
    const [, options] = (fetch as ReturnType<typeof vi.fn>).mock.calls[0] as [
      string,
      RequestInit,
    ];
    expect((options.headers as Record<string, string>)['Accept-Language']).toBe('it');
  });

  it('sends Accept-Language: en when the locale is English', async () => {
    vi.stubGlobal('location', { pathname: '/en/login' });
    await api.login('a@b.com', 'pass');
    const [, options] = (fetch as ReturnType<typeof vi.fn>).mock.calls[0] as [
      string,
      RequestInit,
    ];
    expect((options.headers as Record<string, string>)['Accept-Language']).toBe('en');
  });
});

describe('api — error handling', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('throws ApiError with server message on non-ok response', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: false,
        status: 401,
        statusText: 'Unauthorized',
        json: () => Promise.resolve({ message: 'credenziali non valide' }),
      }),
    );

    await expect(api.login('a@b.com', 'wrong')).rejects.toSatisfy(
      (e: unknown) =>
        e instanceof ApiError && e.status === 401 && e.message === 'credenziali non valide',
    );
  });

  it('falls back to statusText when response body has no message', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: false,
        status: 500,
        statusText: 'Internal Server Error',
        json: () => Promise.reject(new Error('not json')),
      }),
    );

    await expect(api.login('a@b.com', 'pass')).rejects.toSatisfy(
      (e: unknown) => e instanceof ApiError && e.status === 500,
    );
  });
});
