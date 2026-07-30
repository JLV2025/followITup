import { Routes, Route, Navigate } from "react-router-dom";
import { useAuthStore } from "./stores/authStore";
import Dashboard from "./pages/Dashboard";
import Login from "./pages/Login";
import ProjectDetail from "./pages/ProjectDetail";
import TaskListView from "./pages/TaskListView";
import ProjectGantt from "./pages/ProjectGantt";
import Navbar from "./components/Navbar";

function App() {
  const isLoggedIn = useAuthStore((s) => s.isLoggedIn);

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
            <Route path="list" element={<TaskListView />} />
          </Route>
        </Routes>
      </main>
    </div>
  );
}

export default App;
