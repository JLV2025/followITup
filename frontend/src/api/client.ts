import axios from "axios";

const api = axios.create({
  baseURL: "/",
  headers: { "Content-Type": "application/json" },
});

// 请求拦截：附加 token
api.interceptors.request.use((config) => {
  const token = localStorage.getItem("token");
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

// 响应拦截：401 时清除登录状态；403 首登未改密时跳转改密页
api.interceptors.response.use(
  (res) => res,
  (err) => {
    if (err.response?.status === 401) {
      localStorage.removeItem("token");
      localStorage.removeItem("user");
    }
    if (
      err.response?.status === 403 &&
      err.response?.data?.error?.code === "FORCE_PASSWORD_CHANGE" &&
      !err.config?.url?.includes("/api/auth/change-password")
    ) {
      window.location.href = "/change-password";
    }
    return Promise.reject(err);
  }
);

export default api;
