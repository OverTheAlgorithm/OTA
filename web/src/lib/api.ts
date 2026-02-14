export interface User {
  id: string;
  kakao_id: number;
  email?: string;
  nickname?: string;
  profile_image?: string;
  created_at: string;
  updated_at: string;
}

interface ApiResponse<T> {
  data: T;
}

interface ApiError {
  error: string;
}

export async function fetchMe(): Promise<User> {
  const res = await fetch("/api/v1/auth/me", {
    credentials: "include",
  });

  if (!res.ok) {
    const err: ApiError = await res.json();
    throw new Error(err.error || "Failed to fetch user");
  }

  const body: ApiResponse<User> = await res.json();
  return body.data;
}

export async function logout(): Promise<void> {
  await fetch("/api/v1/auth/logout", {
    method: "POST",
    credentials: "include",
  });
}
