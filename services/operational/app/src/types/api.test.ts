import { ApiError } from "./api";

describe("ApiError", () => {
  it("carries status + message", () => {
    const err = new ApiError(401, "nope");
    expect(err.status).toBe(401);
    expect(err.message).toBe("nope");
    expect(err.name).toBe("ApiError");
  });

  it("is catchable as an Error subclass", () => {
    try {
      throw new ApiError(500, "boom");
    } catch (e) {
      expect(e).toBeInstanceOf(Error);
      expect(e).toBeInstanceOf(ApiError);
    }
  });

  it("works with instanceof in branching code", () => {
    const handle = (e: unknown): number => {
      if (e instanceof ApiError) return e.status;
      return -1;
    };
    expect(handle(new ApiError(404, "x"))).toBe(404);
    expect(handle(new Error("not an api error"))).toBe(-1);
  });
});
