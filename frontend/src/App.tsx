import { Routes, Route } from 'react-router-dom';
import { Landing } from './pages/Landing';
import { Dashboard } from './pages/Dashboard';
import { WorkshopView } from './pages/WorkshopView';
import { Join } from './pages/Join';
import { Workspace } from './pages/Workspace';
import { Admin } from './pages/Admin';

export default function App() {
  return (
    <Routes>
      <Route path="/" element={<Landing />} />
      <Route path="/dashboard" element={<Dashboard />} />
      <Route path="/workshop/:id" element={<WorkshopView />} />
      <Route path="/join" element={<Join />} />
      <Route path="/workspace" element={<Workspace />} />
      <Route path="/admin" element={<Admin />} />
    </Routes>
  );
}
