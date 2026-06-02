import { useState } from 'react';
import { Outlet, useLocation } from 'react-router-dom';
import Sidebar from './Sidebar';
import Deploy from '../modals/Deploy';
import Settings from '../modals/Settings';

export default function Layout() {
  const [deployOpen, setDeployOpen] = useState(false);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const { pathname } = useLocation();

  return (
    <div style={{ display: 'flex', height: '100vh', background: 'var(--bg)', boxSizing: 'border-box' }}>
      <Sidebar
        onDeploy={() => setDeployOpen(o => !o)}
        onSettings={() => setSettingsOpen(true)}
        deployActive={deployOpen}
        pathname={pathname}
      />
      <div style={{ flex: 1, display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
        <Outlet />
      </div>
      {deployOpen && <Deploy onClose={() => setDeployOpen(false)} />}
      {settingsOpen && <Settings onClose={() => setSettingsOpen(false)} />}
    </div>
  );
}
