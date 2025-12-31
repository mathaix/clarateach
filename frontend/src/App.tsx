import { Routes, Route } from 'react-router-dom';
import { AuthProvider } from './lib/auth';
import { Landing } from './pages/Landing';
import { Dashboard } from './pages/Dashboard';
import { WorkshopView } from './pages/WorkshopView';
import { Join } from './pages/Join';
import { Register } from './pages/Register';
import { Registered } from './pages/Registered';
import { SessionWorkspace } from './pages/SessionWorkspace';
import { Workspace } from './pages/Workspace';
import { Admin } from './pages/Admin';
import { Login } from './pages/Login';
import { Signup } from './pages/Signup';

export default function App() {
  return (
    <AuthProvider>
      <Routes>
        <Route path="/" element={<Landing />} />
        <Route path="/login" element={<Login />} />
        <Route path="/signup" element={<Signup />} />
        <Route path="/dashboard" element={<Dashboard />} />
        <Route path="/workshop/:id" element={<WorkshopView />} />
        <Route path="/join" element={<Join />} />
        <Route path="/register" element={<Register />} />
        <Route path="/registered/:code" element={<Registered />} />
        <Route path="/s/:code" element={<SessionWorkspace />} />
        <Route path="/workspace" element={<Workspace />} />
        <Route path="/admin" element={<Admin />} />
      </Routes>
    </AuthProvider>
  );
}
