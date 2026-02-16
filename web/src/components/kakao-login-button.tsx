export function KakaoLoginButton() {
  const handleClick = () => {
    window.location.href = "/api/v1/auth/kakao/login";
  };

  return (
    <button
      onClick={handleClick}
      className="flex items-center justify-center gap-2 w-full max-w-xs px-6 py-3 rounded-xl text-sm font-medium transition-colors"
      style={{
        backgroundColor: "#FEE500",
        color: "#000000D9",
      }}
    >
      <svg
        width="18"
        height="18"
        viewBox="0 0 18 18"
        fill="none"
        xmlns="http://www.w3.org/2000/svg"
      >
        <path
          fillRule="evenodd"
          clipRule="evenodd"
          d="M9 0.6C4.029 0.6 0 3.713 0 7.55C0 9.944 1.558 12.06 3.931 13.313L2.933 16.979C2.844 17.301 3.213 17.556 3.494 17.37L7.873 14.434C8.242 14.473 8.618 14.5 9 14.5C13.971 14.5 18 11.387 18 7.55C18 3.713 13.971 0.6 9 0.6Z"
          fill="#000000D9"
        />
      </svg>
      카카오 로그인
    </button>
  );
}
