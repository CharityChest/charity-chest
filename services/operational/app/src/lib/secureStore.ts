import * as SecureStore from "expo-secure-store";

// The single key under which the operational JWT is persisted on the device.
// Picked deliberately short — SecureStore key length limits vary per platform.
const TOKEN_KEY = "cc_op_token";

export async function getToken(): Promise<string | null> {
  return SecureStore.getItemAsync(TOKEN_KEY);
}

export async function setToken(token: string): Promise<void> {
  await SecureStore.setItemAsync(TOKEN_KEY, token);
}

export async function clearToken(): Promise<void> {
  await SecureStore.deleteItemAsync(TOKEN_KEY);
}
