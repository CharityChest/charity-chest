// gRPC client for admin's AdminInternal service. Replaces the HTTP AdminClient
// for service-to-service calls. Authenticates via the "x-service-key" metadata
// entry (constant-time compared by admin's ServiceKeyInterceptor).
//
// The proto file (internal.proto) is a copy of the canonical source at
// proto/admin/internal/v1/internal.proto in the repo root. Regenerate it by
// running `make proto` in services/admin/backend/.

import * as grpc from "@grpc/grpc-js";
import * as protoLoader from "@grpc/proto-loader";
import * as path from "path";

import {
  AdminInvalidCredentialsError,
  AdminUnavailableError,
  AdminUserNotFoundError,
} from "./errors";
import type { AdminApi, UserDTO } from "./types";

// Proto is loaded from alongside this file (both src/ in dev, both dist/ when
// compiled — the build script copies internal.proto to dist/adminclient/).
const PROTO_PATH = path.join(__dirname, "internal.proto");

const packageDefinition = protoLoader.loadSync(PROTO_PATH, {
  keepCase: true,
  longs: String,
  enums: String,
  defaults: true,
  oneofs: true,
});

// eslint-disable-next-line @typescript-eslint/no-explicit-any
const proto = (grpc.loadPackageDefinition(packageDefinition) as any).admin
  .internal.v1;

// Raw response shape from the AdminInternal.UserDTO message.
interface RawUserDTO {
  uuid: string;
  email: string;
  name: string;
  role: string;
  mfa_enabled: boolean;
}

export class AdminGrpcClient implements AdminApi {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  private readonly client: any;
  private readonly baseMeta: grpc.Metadata;
  private readonly timeoutMs: number;

  constructor(grpcUrl: string, serviceApiKey: string, timeoutMs: number) {
    this.client = new proto.AdminInternal(
      grpcUrl,
      grpc.credentials.createInsecure(),
    );
    this.baseMeta = new grpc.Metadata();
    this.baseMeta.set("x-service-key", serviceApiKey);
    this.timeoutMs = timeoutMs;
  }

  login(email: string, password: string, locale?: string): Promise<UserDTO> {
    return this.call<RawUserDTO>("login", { email, password }, locale).then(
      toUserDTO,
    );
  }

  googleAuth(
    googleSub: string,
    email: string,
    name: string,
    locale?: string,
  ): Promise<UserDTO> {
    return this.call<RawUserDTO>(
      "googleAuth",
      { google_sub: googleSub, email, name },
      locale,
    ).then(toUserDTO);
  }

  getUser(userUuid: string, locale?: string): Promise<UserDTO> {
    return this.call<RawUserDTO>("getUser", { user_uuid: userUuid }, locale, {
      notFound: true,
    }).then(toUserDTO);
  }

  private call<T>(
    method: string,
    req: Record<string, string | boolean>,
    locale: string | undefined,
    opts: { notFound?: boolean } = {},
  ): Promise<T> {
    return new Promise((resolve, reject) => {
      const meta = this.baseMeta.clone();
      if (locale) {
        meta.set("x-locale", locale);
      }
      const deadline = new Date(Date.now() + this.timeoutMs);
      this.client[method](
        req,
        meta,
        { deadline },
        (err: grpc.ServiceError | null, response: T) => {
          if (!err) {
            resolve(response);
            return;
          }

          if (
            err.code === grpc.status.UNAUTHENTICATED &&
            !opts.notFound
          ) {
            reject(new AdminInvalidCredentialsError());
            return;
          }
          if (err.code === grpc.status.NOT_FOUND && opts.notFound) {
            reject(new AdminUserNotFoundError());
            return;
          }
          if (
            err.code === grpc.status.DEADLINE_EXCEEDED ||
            err.code === grpc.status.UNAVAILABLE
          ) {
            reject(new AdminUnavailableError(`adminclient: ${err.message}`));
            return;
          }
          reject(new AdminUnavailableError(`adminclient: ${err.message}`));
        },
      );
    });
  }

  /** Close the underlying gRPC channel. Call during graceful shutdown. */
  close(): void {
    this.client.close();
  }
}

function toUserDTO(raw: RawUserDTO): UserDTO {
  return {
    uuid: raw.uuid,
    email: raw.email,
    name: raw.name,
    // proto3 strings default to ""; treat empty as absent to match the HTTP
    // client's behaviour (role?: string | null).
    role: raw.role !== "" ? raw.role : undefined,
    mfa_enabled: raw.mfa_enabled,
  };
}
