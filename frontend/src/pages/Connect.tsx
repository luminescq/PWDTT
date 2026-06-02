import { useState, useEffect } from 'react';
import { IconCloverFilled, IconPlus, IconChevronUp, IconPencil } from '@tabler/icons-react';
import AddServer from '../modals/Add-server';
import EditServer from '../modals/Edit-server';
import { serverStore } from '../lib/store';
import { tunnelStore } from '../lib/stores/tunnelStore';
import { settingsStore } from '../lib/store';
import { themeStore } from '../lib/stores/themeStore';
import { toastStore } from '../lib/stores/toastStore';
import { wdttLinkStore } from '../lib/utils/wdttLink';
import { SaveProfile } from '../../wailsjs/go/backend/App';
import type { Server, TunnelState } from '../lib/types';
import { Connect as WailsConnect, Disconnect as WailsDisconnect } from '../../wailsjs/go/backend/App';
import shapeLight from '../assets/shape-light.png';
import shapeDark from '../assets/shape-dark.png';
import powerIcon from '../assets/power-icon.png';

const PING_COLORS: Record<string, string> = {
  good: '#22c55e',
  mid: '#f59e0b',
  bad: '#ef4444',
  none: 'var(--border)',
};

function pingColor(ping?: number) {
  if (!ping) return PING_COLORS.none;
  if (ping < 100) return PING_COLORS.good;
  if (ping < 200) return PING_COLORS.mid;
  return PING_COLORS.bad;
}

const TUNNEL_LABEL: Record<TunnelState, string> = {
  idle: 'Подключить',
  connecting: 'Подключение...',
  connected: 'Отключить',
  disconnecting: 'Отключение...',
};

export default function Connect() {
  const [servers, setServers] = useState<Server[]>(() => serverStore.getAll());
  const [selected, setSelected] = useState<Server | null>(() => {
    const all = serverStore.getAll();
    return all.length > 0 ? all[0] : null;
  });
  const [listOpen, setListOpen] = useState(false);

  // tunnelState из глобального store — переживает смену роута
  const [tunnelState, setTunnelState] = useState<TunnelState>(() => tunnelStore.get());
  useEffect(() => tunnelStore.subscribe(setTunnelState), []);

  const [addServerOpen, setAddServerOpen] = useState(false);
  const [editServer, setEditServer] = useState<Server | null>(null);
  const [theme, setTheme] = useState(() => themeStore.get());
  useEffect(() => themeStore.subscribe(setTheme), []);

  const [linkFlash, setLinkFlash] = useState(false);

  useEffect(() => {
    return wdttLinkStore.subscribe((link) => {
      if (!link) return;
      const consumed = wdttLinkStore.consume();
      if (!consumed) return;
      const host = `${consumed.ip}:${consumed.dtlsPort}`;
      const name = consumed.name;

      const finish = async (saveHashes: boolean) => {
        await SaveProfile(name, {
          peer: host,
          password: consumed.password,
          hashes: saveHashes ? consumed.hashes : [],
          turn: '', port: '', device_id: '', listen: '',
        });
        // не дублируем если сервер с таким host уже есть
        const existing = serverStore.getAll().find(s => s.host === host);
        const s = existing ?? serverStore.add({ name, host, password: consumed.password });
        setServers(serverStore.getAll());
        setSelected(s);
        setLinkFlash(true);
        setTimeout(() => setLinkFlash(false), 800);
        toastStore.show(existing ? `Профиль обновлён: ${name}` : `Профиль добавлен: ${name}`, 3000);
        if (saveHashes) {
          const settings = settingsStore.get();
          settingsStore.save({ ...settings, hashes: consumed.hashes.slice(0, 4) as [string,string,string,string] });
        }
      };

      if (consumed.hashes.length > 0) {
        const yes = window.confirm(`Ссылка содержит хеши. Перезаписать текущие хеши?`);
        finish(yes);
      } else {
        finish(false);
      }
    });
  }, []);

  const doConnect = async () => {
    const s = settingsStore.get();
    const hashes = s.hashes.filter(h => h.trim());
    if (hashes.length === 0) {
      toastStore.show('Добавьте хеши в Настройках');
      return;
    }
    tunnelStore.set('connecting');
    try {
      await WailsConnect({
        profile: selected!.name,
        captchaMode: 'auto',
        workers: s.power || 9,
        mtu: s.mtu || 1280,
        hashes,
      });
    } catch {
      tunnelStore.set('idle');
    }
  };

  const [reconnectAt, setReconnectAt] = useState(0); // timestamp когда можно снова подключиться

  const handleTunnel = async () => {
    if (!selected) return;
    if (tunnelState === 'idle') {
      if (Date.now() < reconnectAt) {
        const secs = Math.ceil((reconnectAt - Date.now()) / 1000);
        toastStore.show(`Подождите ${secs} сек.`, 2000);
        return;
      }
      toastStore.show('Убедитесь что другие VPN отключены', 4000);
      await doConnect();
    } else if (tunnelState === 'connected' || tunnelState === 'connecting') {
      tunnelStore.set('disconnecting');
      await WailsDisconnect();
      tunnelStore.set('idle');
      setReconnectAt(Date.now() + 4000);
    }
  };

  const handleAdd = (data: Omit<Server, 'id'>) => {
    const s = serverStore.add(data);
    setServers(serverStore.getAll());
    setSelected(s);
  };

  const handleSave = (server: Server) => {
    serverStore.update(server);
    const all = serverStore.getAll();
    setServers(all);
    if (selected?.id === server.id) setSelected(server);
  };

  const handleDelete = (id: string) => {
    serverStore.remove(id);
    const all = serverStore.getAll();
    setServers(all);
    if (selected?.id === id) setSelected(all[0] ?? null);
  };

  const isActive = tunnelState === 'connected';
  const isSpinning = tunnelState === 'connecting' || tunnelState === 'disconnecting';
  const isBusy = tunnelState === 'disconnecting';

  return (
    <>
      <style>{`
        * { font-family: 'Geist', sans-serif; font-weight: 500; box-sizing: border-box; }
        .main { flex: 1; position: relative; display: flex; align-items: center; justify-content: center; animation: page-in 0.25s ease-out; background: var(--bg); }
        .btn-add { position: absolute; top: 16px; right: 20px; background: none; border: none; cursor: pointer; color: var(--text); }
        .power-btn { position: relative; width: 160px; height: 160px; background: none; border: none; cursor: pointer; display: flex; align-items: center; justify-content: center; padding: 0; transition: opacity 0.2s; }
        .power-btn:disabled { opacity: 0.5; cursor: not-allowed; }
        .orb { position: absolute; width: 130px; height: 130px; }
        .orb img { width: 100%; height: 100%; display: block; }
        .orb--spinning { animation: shape-spin 2s linear infinite; }
        .orb--active { animation: shape-pulse 1.2s ease-in-out infinite; }
        @keyframes shape-spin { from { transform: rotate(0deg); } to { transform: rotate(360deg); } }
        @keyframes shape-pulse { 0%,100% { transform: scale(1); } 50% { transform: scale(1.1); } }
        @keyframes link-flash { 0% { opacity:1; } 30% { opacity:0.2; } 60% { opacity:1; } 80% { opacity:0.4; } 100% { opacity:1; } }
        .orb--flash { animation: link-flash 0.8s ease-out; }
        .power-icon { position: relative; z-index: 1; display: flex; align-items: center; justify-content: center; }
        .status-bar { position: absolute; bottom: 24px; left: 50%; transform: translateX(-50%); display: flex; flex-direction: column; align-items: stretch; width: 380px; }
        .server-list { border: 1px solid var(--border); border-radius: 12px; overflow: hidden; margin-bottom: 8px; background: var(--surface); animation: slide-down 0.28s ease-out; }
        .server-item { display: flex; align-items: center; gap: 10px; width: 100%; padding: 12px 20px; background: var(--bg-2); font-size: 15px; color: var(--text); font-family: 'Geist', sans-serif; font-weight: 500; border-bottom: 1px solid var(--border-2); }
        .server-item:last-child { border-bottom: none; }
        .server-item:hover { background: var(--bg-3); }
        .server-item--active { background: var(--bg-3); }
        .server-icon-btn { background: none; border: none; cursor: pointer; padding: 0; display: flex; align-items: center; color: var(--text); }
        .server-edit-btn { background: none; border: none; cursor: pointer; padding: 0; display: flex; align-items: center; color: var(--text-3); opacity: 0; transition: opacity 0.15s; }
        .server-item:hover .server-edit-btn { opacity: 1; }
        .status-server { display: flex; align-items: center; gap: 10px; background: var(--surface); border: 1px solid var(--border); border-radius: 12px; padding: 10px 20px; font-size: 15px; color: var(--text); cursor: pointer; width: 100%; font-family: 'Geist', sans-serif; font-weight: 500; }
        .status-server--empty { color: var(--text-4); }
        .status-name { flex: 1; text-align: left; }
        .status-ping { display: flex; align-items: center; gap: 6px; font-size: 14px; }
        .ping-dot { width: 8px; height: 8px; border-radius: 50%; }
        .tunnel-label { position: absolute; top: 50%; left: 50%; transform: translate(-50%, calc(-50% + 80px)); font-size: 13px; color: var(--text-3); pointer-events: none; }
      `}</style>
      <main className="main">
        <button className="btn-add" onClick={() => setAddServerOpen(true)}>
          <IconPlus stroke={2} size={22} />
        </button>

        <button
          className="power-btn"
          onClick={handleTunnel}
          disabled={!selected || isBusy}
          title={selected ? TUNNEL_LABEL[tunnelState] : 'Добавьте сервер'}
        >
          <div className={`orb${isSpinning ? ' orb--spinning' : isActive ? ' orb--active' : ''}${linkFlash ? ' orb--flash' : ''}`}>
            <img src={theme === 'dark' ? shapeDark : shapeLight} alt="" draggable={false} />
          </div>
          <div className="power-icon">
            <img src={powerIcon} alt="" draggable={false} style={{ width: 28, height: 35 }} />
          </div>
        </button>

        <span className="tunnel-label">{selected ? TUNNEL_LABEL[tunnelState] : 'Нет серверов'}</span>

        <div className="status-bar">
          {listOpen && servers.length > 0 && (
            <div className="server-list">
              {servers.map(s => (
                <div key={s.id} className={`server-item${s.id === selected?.id ? ' server-item--active' : ''}`}>
                  <button className="server-icon-btn" onClick={() => setEditServer(s)}>
                    <IconCloverFilled size={20} />
                  </button>
                  <span
                    className="status-name"
                    style={{ cursor: 'pointer' }}
                    onClick={() => { setSelected(s); setListOpen(false); }}
                  >
                    {s.name}
                  </span>
                  {s.ping != null && (
                    <span className="status-ping">
                      <span className="ping-dot" style={{ background: pingColor(s.ping) }} />
                      {s.ping}
                    </span>
                  )}
                  <button className="server-edit-btn" onClick={() => setEditServer(s)}>
                    <IconPencil size={15} stroke={2} />
                  </button>
                </div>
              ))}
            </div>
          )}

          <button className={`status-server${!selected ? ' status-server--empty' : ''}`} onClick={() => setListOpen(o => !o)}>
            <IconCloverFilled size={20} />
            <span className="status-name">{selected ? selected.name : 'Нет серверов'}</span>
            {selected?.ping != null && (
              <span className="status-ping">
                <span className="ping-dot" style={{ background: pingColor(selected.ping) }} />
                {selected.ping}
              </span>
            )}
            <IconChevronUp
              size={16}
              style={{ transform: listOpen ? 'rotate(0deg)' : 'rotate(180deg)', transition: 'transform 0.2s' }}
            />
          </button>
        </div>

        {addServerOpen && <AddServer onClose={() => setAddServerOpen(false)} onAdd={handleAdd} />}
        {editServer && (
          <EditServer
            server={editServer}
            onClose={() => setEditServer(null)}
            onSave={handleSave}
            onDelete={handleDelete}
          />
        )}
      </main>
    </>
  );
}
