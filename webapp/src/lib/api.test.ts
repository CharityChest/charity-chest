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

describe('api — X-Locale header', () => {
  beforeEach(() => {
    vi.stubGlobal('location', { pathname: '/it/login' });
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve({
            data: {
              token: 'tok',
              user: { id: 1, email: 'a@b.com', name: 'A', created_at: '', updated_at: '' },
            },
          }),
      }),
    );
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('sends X-Locale: it when the locale is Italian', async () => {
    await api.login('a@b.com', 'pass');
    const [, options] = (fetch as ReturnType<typeof vi.fn>).mock.calls[0] as [
      string,
      RequestInit,
    ];
    expect((options.headers as Record<string, string>)['X-Locale']).toBe('it');
  });

  it('sends X-Locale: en when the locale is English', async () => {
    vi.stubGlobal('location', { pathname: '/en/login' });
    await api.login('a@b.com', 'pass');
    const [, options] = (fetch as ReturnType<typeof vi.fn>).mock.calls[0] as [
      string,
      RequestInit,
    ];
    expect((options.headers as Record<string, string>)['X-Locale']).toBe('en');
  });
});

// --- New ACL methods ---

describe('api — systemStatus', () => {
  afterEach(() => { vi.unstubAllGlobals(); });

  it('calls GET /v1/system/status with no auth header', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ data: { configured: true } }),
    }));
    const result = await api.systemStatus();
    expect(result).toEqual({ configured: true });
    const [url, opts] = (fetch as ReturnType<typeof vi.fn>).mock.calls[0] as [string, RequestInit];
    expect(url).toContain('/v1/system/status');
    expect((opts.headers as Record<string, string>)['Authorization']).toBeUndefined();
  });
});

describe('api — assignSystemRole', () => {
  afterEach(() => { vi.unstubAllGlobals(); });

  it('calls POST /v1/api/system/assign-role with the token and body', async () => {
    localStorage.setItem('cc_token', 'root-jwt');
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ data: { id: 5, email: 'u@u.com', name: 'U', role: 'system', created_at: '', updated_at: '' } }),
    }));
    await api.assignSystemRole(5, 'system');
    const [url, opts] = (fetch as ReturnType<typeof vi.fn>).mock.calls[0] as [string, RequestInit];
    expect(url).toContain('/v1/api/system/assign-role');
    expect(opts.method).toBe('POST');
    expect((opts.headers as Record<string, string>)['Authorization']).toBe('Bearer root-jwt');
    expect(JSON.parse(opts.body as string)).toEqual({ user_id: 5, role: 'system' });
    localStorage.clear();
  });
});

describe('api — org CRUD', () => {
  afterEach(() => { vi.unstubAllGlobals(); localStorage.clear(); });

  beforeEach(() => {
    localStorage.setItem('cc_token', 'sys-jwt');
  });

  function mockFetch(body: unknown) {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ data: body }),
    }));
  }

  it('listOrgs calls GET /v1/api/orgs', async () => {
    mockFetch([]);
    await api.listOrgs();
    const [url, opts] = (fetch as ReturnType<typeof vi.fn>).mock.calls[0] as [string, RequestInit];
    expect(url).toContain('/v1/api/orgs');
    expect(opts.method).toBeUndefined(); // GET is default
    expect((opts.headers as Record<string, string>)['Authorization']).toBe('Bearer sys-jwt');
  });

  it('createOrg calls POST /v1/api/orgs with name', async () => {
    mockFetch({ id: 1, name: 'Org A', created_at: '', updated_at: '' });
    await api.createOrg('Org A');
    const [url, opts] = (fetch as ReturnType<typeof vi.fn>).mock.calls[0] as [string, RequestInit];
    expect(url).toContain('/v1/api/orgs');
    expect(opts.method).toBe('POST');
    expect(JSON.parse(opts.body as string)).toEqual({ name: 'Org A' });
  });

  it('getOrg calls GET /v1/api/orgs/3', async () => {
    mockFetch({ id: 3, name: 'Org C', created_at: '', updated_at: '' });
    await api.getOrg(3);
    const [url] = (fetch as ReturnType<typeof vi.fn>).mock.calls[0] as [string, RequestInit];
    expect(url).toContain('/v1/api/orgs/3');
  });

  it('updateOrg calls PUT /v1/api/orgs/3 with name', async () => {
    mockFetch({ id: 3, name: 'New Name', created_at: '', updated_at: '' });
    await api.updateOrg(3, 'New Name');
    const [url, opts] = (fetch as ReturnType<typeof vi.fn>).mock.calls[0] as [string, RequestInit];
    expect(url).toContain('/v1/api/orgs/3');
    expect(opts.method).toBe('PUT');
    expect(JSON.parse(opts.body as string)).toEqual({ name: 'New Name' });
  });

  it('deleteOrg calls DELETE /v1/api/orgs/3', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, status: 204, json: () => Promise.resolve(null) }));
    await api.deleteOrg(3);
    const [url, opts] = (fetch as ReturnType<typeof vi.fn>).mock.calls[0] as [string, RequestInit];
    expect(url).toContain('/v1/api/orgs/3');
    expect(opts.method).toBe('DELETE');
  });
});

describe('api — member management', () => {
  afterEach(() => { vi.unstubAllGlobals(); localStorage.clear(); });

  beforeEach(() => {
    localStorage.setItem('cc_token', 'owner-jwt');
  });

  function mockFetch(body: unknown) {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ data: body }),
    }));
  }

  it('listMembers calls GET /v1/api/orgs/7/members', async () => {
    mockFetch([]);
    await api.listMembers(7);
    const [url] = (fetch as ReturnType<typeof vi.fn>).mock.calls[0] as [string, RequestInit];
    expect(url).toContain('/v1/api/orgs/7/members');
  });

  it('addMember calls POST /v1/api/orgs/7/members with user_id and role', async () => {
    mockFetch({ id: 1, org_id: 7, user_id: 9, role: 'operational', created_at: '', updated_at: '' });
    await api.addMember(7, 9, 'operational');
    const [url, opts] = (fetch as ReturnType<typeof vi.fn>).mock.calls[0] as [string, RequestInit];
    expect(url).toContain('/v1/api/orgs/7/members');
    expect(opts.method).toBe('POST');
    expect(JSON.parse(opts.body as string)).toEqual({ user_id: 9, role: 'operational' });
  });

  it('updateMember calls PUT /v1/api/orgs/7/members/9 with role', async () => {
    mockFetch({ id: 1, org_id: 7, user_id: 9, role: 'admin', created_at: '', updated_at: '' });
    await api.updateMember(7, 9, 'admin');
    const [url, opts] = (fetch as ReturnType<typeof vi.fn>).mock.calls[0] as [string, RequestInit];
    expect(url).toContain('/v1/api/orgs/7/members/9');
    expect(opts.method).toBe('PUT');
    expect(JSON.parse(opts.body as string)).toEqual({ role: 'admin' });
  });

  it('removeMember calls DELETE /v1/api/orgs/7/members/9', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, status: 204, json: () => Promise.resolve(null) }));
    await api.removeMember(7, 9);
    const [url, opts] = (fetch as ReturnType<typeof vi.fn>).mock.calls[0] as [string, RequestInit];
    expect(url).toContain('/v1/api/orgs/7/members/9');
    expect(opts.method).toBe('DELETE');
  });
});

describe('api — searchUsers', () => {
  afterEach(() => { vi.unstubAllGlobals(); localStorage.clear(); });

  beforeEach(() => {
    localStorage.setItem('cc_token', 'root-jwt');
  });

  const paginatedResponse = {
    data: [
      { id: 1, email: 'alice@example.com', name: 'Alice', role: null, mfa_enabled: false, created_at: '', updated_at: '', organizations: [] },
    ],
    metadata: { page: 1, size: 20, total: 1, total_pages: 1 },
  };

  it('calls GET /v1/api/admin/users with email, page, and size query params', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(paginatedResponse),
    }));
    await api.searchUsers('alice', 1, 20);
    const [url] = (fetch as ReturnType<typeof vi.fn>).mock.calls[0] as [string, RequestInit];
    expect(url).toContain('/v1/api/admin/users');
    expect(url).toContain('email=alice');
    expect(url).toContain('page=1');
    expect(url).toContain('size=20');
  });

  it('sends Authorization header with bearer token', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(paginatedResponse),
    }));
    await api.searchUsers('', 1, 20);
    const [, opts] = (fetch as ReturnType<typeof vi.fn>).mock.calls[0] as [string, RequestInit];
    expect((opts.headers as Record<string, string>)['Authorization']).toBe('Bearer root-jwt');
  });

  it('returns the full { data, metadata } object', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(paginatedResponse),
    }));
    const result = await api.searchUsers('alice', 1, 20);
    expect(result.data).toHaveLength(1);
    expect(result.metadata.total).toBe(1);
    expect(result.metadata.total_pages).toBe(1);
  });

  it('omits email param when email is empty string', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ data: [], metadata: { page: 1, size: 20, total: 0, total_pages: 1 } }),
    }));
    await api.searchUsers('', 1, 20);
    const [url] = (fetch as ReturnType<typeof vi.fn>).mock.calls[0] as [string, RequestInit];
    expect(url).not.toContain('email=');
  });

  it('throws ApiError on 403', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: false,
      status: 403,
      statusText: 'Forbidden',
      json: () => Promise.resolve({ message: 'forbidden' }),
    }));
    await expect(api.searchUsers('x', 1, 20)).rejects.toSatisfy(
      (e: unknown) => e instanceof ApiError && e.status === 403,
    );
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
