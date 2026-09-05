export type User = { id: string; email: string; display_name: string };
export type Project = {
  id: string;
  slug: string;
  name: string;
  role: "owner" | "maintainer" | "developer" | "viewer";
};
export type Environment = {
  id: string;
  slug: string;
  name: string;
  kind: "development" | "staging" | "production";
};
export class APIError extends Error {
  constructor(public status: number) {
    super(
      (
        {
          400: "请检查输入内容。",
          401: "登录已失效，请重新登录。",
          403: "当前角色没有操作权限。",
          404: "项目不存在或你已不再是项目成员。",
          409: "标识已存在，请使用其他标识。",
          429: "登录尝试过于频繁，请一分钟后重试。",
        } as Record<number, string>
      )[status] || "服务暂时不可用，请稍后重试。",
    );
  }
}
export async function request<T>(
  path: string,
  token: string | null,
  options: RequestInit = {},
): Promise<T> {
  let response: Response;
  try {
    response = await fetch(`/api/v2${path}`, {
      ...options,
      signal: AbortSignal.any([
        AbortSignal.timeout(20000),
        ...(options.signal ? [options.signal] : []),
      ]),
      headers: {
        "Content-Type": "application/json",
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
        ...options.headers,
      },
    });
  } catch (error) {
    if (error instanceof Error && error.name === "AbortError") throw error;
    throw new Error("无法连接平台服务，请检查网络后重试。");
  }
  if (!response.ok) throw new APIError(response.status);
  return response.status === 204
    ? (undefined as T)
    : (response.json() as Promise<T>);
}
