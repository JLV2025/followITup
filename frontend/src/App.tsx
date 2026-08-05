import { useEffect } from "react";
import { Routes, Route, Navigate } from "react-router-dom";
import { useAuthStore } from "./stores/authStore";
import Dashboard from "./pages/Dashboard";
import Login from "./pages/Login";
import ChangePassword from "./pages/ChangePassword";
import ProjectDetail from "./pages/ProjectDetail";
import ProjectGantt from "./pages/ProjectGantt";
import UserManagement from "./pages/UserManagement";
import SystemSettings from "./pages/SystemSettings";
import Navbar from "./components/Navbar";

function App() {
  const isLoggedIn = useAuthStore((s) => s.isLoggedIn);
  const loadFromStorage = useAuthStore((s) => s.loadFromStorage);

  // 全局恢复登录态：任何页面整页加载/刷新都生效（原只在 Dashboard/ProjectGantt 挂载时执行，直达非首页会丢登录态）
  useEffect(() => {
    loadFromStorage();
  }, [loadFromStorage]);

  return (
    <div className="app">
      <Navbar />
      <main className="main-content">
        <Routes>
          <Route path="/" element={<Dashboard />} />
          <Route
            path="/login"
            element={isLoggedIn ? <Navigate to="/" replace /> : <Login />}
          />
          <Route path="/project/:id" element={<ProjectDetail />}>
            <Route
              index
              element={<ProjectGantt readonly={!isLoggedIn} />}
            />
          </Route>
          <Route path="/admin/users" element={<UserManagement />} />
          <Route path="/admin/settings" element={<SystemSettings />} />
          <Route path="/change-password" element={<ChangePassword />} />
        </Routes>
      </main>
    </div>
  );
}

export default App;
