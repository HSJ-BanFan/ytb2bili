/**
 * 带认证的 fetch 函数包装器
 * 自动从 localStorage 获取 JWT token 并添加到请求头
 */
export async function authFetch(url: string, options: RequestInit = {}): Promise<Response> {
    const jwtToken = localStorage.getItem("jwt_token");

    const headers: HeadersInit = {
        'Content-Type': 'application/json',
        ...options.headers,
    };

    if (jwtToken) {
        (headers as Record<string, string>)['Authorization'] = `Bearer ${jwtToken}`;
    }

    return fetch(url, { ...options, headers });
}
