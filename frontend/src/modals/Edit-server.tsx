import { useState } from 'react';
import { IconCircleHalf2 } from '@tabler/icons-react';
import type { Server } from '../lib/types';
import { SaveProfile, DeleteProfile, GetProfile } from '../../wailsjs/go/backend/App';

interface Props {
  server: Server;
  onClose: () => void;
  onSave: (server: Server) => void;
  onDelete: (id: string) => void;
}

export default function EditServer({ server, onClose, onSave, onDelete }: Props) {
  const [name, setName] = useState(server.name);
  const [ip, port0] = server.host.includes(':') ? server.host.split(':') : [server.host, '56000'];
  const [serverIp, setServerIp] = useState(ip);
  const [serverPort, setServerPort] = useState(port0);
  const [password, setPassword] = useState(server.password);

  const handleSave = async () => {
    if (!name.trim() || !serverIp.trim()) return;
    const updated: Server = {
      ...server,
      name: name.trim(),
      host: `${serverIp.trim()}:${serverPort.trim() || '56000'}`,
      password,
    };
    // читаем существующие хеши чтобы не затереть
    const existing = await GetProfile(server.name).catch(() => null);
    const hashes = existing?.hashes ?? [];
    if (server.name !== updated.name) {
      await DeleteProfile(server.name).catch(() => {});
    }
    await SaveProfile(updated.name, {
      peer: updated.host,
      password: updated.password,
      hashes,
      turn: '',
      port: '',
      device_id: '',
      listen: '',
    });
    onSave(updated);
    onClose();
  };

  const handleDelete = async () => {
    await DeleteProfile(server.name).catch(() => {});
    onDelete(server.id);
    onClose();
  };

  return (
    <>
      <style>{`
        .es-overlay { position: fixed; inset: 0; background: var(--overlay-bg); backdrop-filter: blur(4px); display: flex; align-items: center; justify-content: center; z-index: 100; animation: overlay-in 0.3s ease-out; }
        .es-modal { background: var(--surface); border-radius: 14px; padding: 20px; width: 380px; max-width: 95vw; box-shadow: var(--shadow); border: 1px solid var(--border); max-height: 90vh; overflow-y: auto; animation: modal-in 0.3s ease-out; }
        .es-header { display: flex; align-items: center; gap: 10px; margin-bottom: 18px; color: var(--text); }
        .es-title { font-size: 16px; font-weight: 600; flex: 1; color: var(--text); }
        .es-close { background: none; border: none; cursor: pointer; font-size: 18px; color: var(--text); line-height: 1; padding: 0; }
        .es-input { width: 100%; padding: 11px 14px; border: 1.5px solid var(--input-border); border-radius: 10px; font-size: 14px; font-family: 'Geist', sans-serif; outline: none; margin-bottom: 10px; box-sizing: border-box; color: var(--text); background: var(--input-bg); }
        .es-input::placeholder { color: var(--text-4); }
        .es-textarea { width: 100%; padding: 10px 14px; border: 1.5px solid var(--input-border); border-radius: 10px; font-size: 13px; font-family: 'Geist Mono', monospace; outline: none; margin-bottom: 10px; box-sizing: border-box; color: var(--text); background: var(--input-bg); resize: vertical; min-height: 70px; }
        .es-textarea::placeholder { color: var(--text-4); }
        .es-label { font-size: 12px; color: var(--text-3); margin-bottom: 5px; display: block; }
        .es-btn-row { display: flex; gap: 10px; margin-top: 4px; }
        .es-btn { flex: 1; padding: 13px; border: none; border-radius: 10px; font-size: 14px; font-family: 'Geist', sans-serif; font-weight: 600; cursor: pointer; }
        .es-btn--save { background: var(--accent); color: var(--accent-fg); }
        .es-btn--save:disabled { opacity: 0.4; cursor: not-allowed; }
        .es-btn--delete { background: #cc0000; color: #fff; }
      `}</style>
      <div className="es-overlay" onClick={onClose}>
        <div className="es-modal" onClick={e => e.stopPropagation()}>
          <div className="es-header">
            <IconCircleHalf2 size={22} />
            <span className="es-title">Редактирование сервера</span>
            <button className="es-close" onClick={onClose}>✕</button>
          </div>
          <input className="es-input" placeholder="Название сервера" value={name} onChange={e => setName(e.target.value)} />
          <div style={{ display: 'flex', gap: 8 }}>
            <input className="es-input" style={{ flex: 1 }} placeholder="IP сервера" value={serverIp} onChange={e => setServerIp(e.target.value)} />
            <input className="es-input" style={{ width: 100 }} placeholder="Порт" value={serverPort} onChange={e => setServerPort(e.target.value)} />
          </div>
          <input className="es-input" placeholder="Пароль туннеля" type="password" value={password} onChange={e => setPassword(e.target.value)} />
          <div className="es-btn-row">
            <button className="es-btn es-btn--save" onClick={handleSave} disabled={!name.trim() || !serverIp.trim()}>Сохранить</button>
            <button className="es-btn es-btn--delete" onClick={handleDelete}>Удалить</button>
          </div>
        </div>
      </div>
    </>
  );
}
