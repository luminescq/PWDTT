import { useState } from 'react';
import { useNavigate, useLocation } from 'react-router-dom';
import {
  IconPlugConnected,
  IconServer2,
  IconTerminal2,
  IconSettings2,
  IconSun,
  IconMoon,
} from '@tabler/icons-react';
import { themeStore } from '../lib/stores/themeStore';

const NAV = [
  { path: '/', icon: <IconPlugConnected stroke={2} size={22} /> },
  { path: '/logs', icon: <IconTerminal2 stroke={2} size={22} /> },
];

interface Props {
  onDeploy?: () => void;
  onSettings?: () => void;
  deployActive?: boolean;
  pathname?: string;
}

export default function Sidebar({ onDeploy, onSettings, deployActive, pathname: pathnameProp }: Props) {
  const navigate = useNavigate();
  const location = useLocation();
  const pathname = pathnameProp ?? location.pathname;
  const [theme, setTheme] = useState(() => themeStore.get());

  const toggleTheme = () => {
    themeStore.toggle();
    setTheme(themeStore.get());
  };

  return (
    <>
      <style>{`
        .sidebar { width: 70px; background: linear-gradient(to bottom, var(--sidebar-from), var(--sidebar-to)); border-radius: 12px; margin: 2px; display: flex; flex-direction: column; justify-content: space-between; padding: 16px 0; overflow: hidden; flex-shrink: 0; }
        .sidebar-top, .sidebar-bottom { display: flex; flex-direction: column; align-items: center; gap: 8px; }
        .nav-btn { width: 48px; height: 48px; border: none; border-radius: 12px; background: transparent; color: #fff; cursor: pointer; display: flex; align-items: center; justify-content: center; opacity: 0.75; transition: opacity 0.15s; }
        .nav-btn:hover { opacity: 1; }
        .nav-btn--active { background: var(--sidebar-btn-active); opacity: 1; border-radius: 16px 16px 16px 2px; }
        .nav-btn--active-plain { background: var(--sidebar-btn-active); opacity: 1; border-radius: 16px 16px 2px 16px; }
        .theme-toggle { background: none; border: none; cursor: pointer; padding: 4px; display: flex; align-items: center; justify-content: center; color: #fff; opacity: 0.7; border-radius: 8px; transition: opacity 0.15s; }
        .theme-toggle:hover { opacity: 1; }
        .theme-toggle svg { transition: transform 0.35s ease, opacity 0.35s ease; }
        .theme-toggle:active svg { transform: rotate(180deg) scale(0.8); opacity: 0.4; }
        @keyframes icon-swap { 0% { transform: rotate(-90deg) scale(0.5); opacity: 0; } 100% { transform: rotate(0deg) scale(1); opacity: 1; } }
        .theme-toggle svg { animation: icon-swap 0.3s ease-out; }
        .theme-toggle--dark .theme-toggle-thumb svg { color: #818cf8; }
      `}</style>
      <aside className="sidebar">
        <div className="sidebar-top">
          {NAV.map(({ path, icon }) => (
            <button
              key={path}
              className={`nav-btn${!deployActive && pathname === path ? ' nav-btn--active' : ''}`}
              onClick={() => navigate(path)}
            >
              {icon}
            </button>
          ))}
          <button className={`nav-btn${deployActive ? ' nav-btn--active-plain' : ''}`} onClick={onDeploy}>
            <IconServer2 stroke={2} size={22} />
          </button>
        </div>
        <div className="sidebar-bottom">
          <button className="theme-toggle" onClick={toggleTheme} title="Toggle theme">
            {theme === 'light' ? <IconMoon size={17} stroke={2} /> : <IconSun size={17} stroke={2} />}
          </button>
          <button className="nav-btn" onClick={onSettings}>
            <IconSettings2 stroke={2} size={22} />
          </button>
        </div>
      </aside>
    </>
  );
}
