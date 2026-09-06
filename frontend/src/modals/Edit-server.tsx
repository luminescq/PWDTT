import { useState } from 'react';
import type React from 'react';
import { IconCircleHalf2, IconHash, IconEye, IconEyeOff, IconX } from '@tabler/icons-react';
import type { Server } from '../lib/types';
import Hash from './Hash';
import { SaveProfile, DeleteProfile } from '../../wailsjs/go/backend/App';
import { toastStore } from '../lib/stores/toastStore';

interface Props {
  server: Server;
  onClose: () => void;
  onSave: (server: Server) => void;
  onDelete: (id: string) => void;
}

export default function EditServer({ server, onClose, onSave, onDelete }: Props) {
  const [name, setName] = useState(server.name);
  const lastColon = server.host.lastIndexOf(':');
  const ip = lastColon !== -1 ? server.host.slice(0, lastColon) : server.host;
  const port0 = lastColon !== -1 ? server.host.slice(lastColon + 1) : '56000';
  const [serverIp, setServerIp] = useState(ip);
  const [serverPort, setServerPort] = useState(port0);
  const [password, setPassword] = useState(server.password);
  const [showPassword, setShowPassword] = useState(false);
  const [hashes, setHashes] = useState<[string,string,string,string]>(
    server.hashes ?? ['', '', '', '']
  );
  const [hashOpen, setHashOpen] = useState(false);

  const filledHashes = hashes.filter(h => h.trim()).length;
  const powerMax = Math.max(9, filledHashes * 27);
  const [power, setPower] = useState<number>(server.power ?? Math.max(9, filledHashes * 9));

  const handleHashSave = (h: [string, string, string, string]) => {
    setHashes(h);
    const filled = h.filter(x => x.trim()).length;
    const newMax = Math.max(9, filled * 27);
    setPower(p => Math.min(p, newMax) || Math.max(9, filled * 9));
  };

  const [saving, setSaving] = useState(false);

  const handleSave = async () => {
    if (!name.trim() || !serverIp.trim() || saving) return;
    setSaving(true);
    const updated: Server = {
      ...server,
      name: name.trim(),
      host: `${serverIp.trim()}:${serverPort.trim() || '56000'}`,
      password,
      hashes,
      power,
    };
    // Сначала сохраняем новый, потом удаляем старый
    try {
      await SaveProfile(updated.name, {
        peer: updated.host,
        password: updated.password,
        hashes,
        turn: '', port: '', device_id: '', listen: '',
        turn_tcp: false,
      });
    } catch {
      // Не двигаем UI-состояние: иначе localStorage разойдётся с диском,
      // а авто-реимпорт профилей создаст дубликат
      toastStore.show('Ошибка сохранения', 3000);
      setSaving(false);
      return;
    }
    if (server.name !== updated.name) {
      await DeleteProfile(server.name).catch(() => { toastStore.show('Ошибка удаления', 3000); });
    }
    onSave(updated);
    setSaving(false);
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
        .es-hash-btn { width: 100%; margin-top: 4px; margin-bottom: 10px; padding: 13px; border: 1.5px solid var(--border); border-radius: 10px; background: var(--surface); color: var(--text); font-size: 14px; font-family: 'Geist', sans-serif; font-weight: 600; cursor: pointer; display: flex; align-items: center; justify-content: center; gap: 8px; transition: border-color 0.15s, box-shadow 0.15s; }
        .es-hash-btn:hover { border-color: var(--accent); box-shadow: 0 0 16px var(--accent-dim, rgba(139,92,246,0.2)); }
        .es-btn-row { display: flex; gap: 10px; margin-top: 4px; }
        .es-btn { flex: 1; padding: 13px; border: none; border-radius: 10px; font-size: 14px; font-family: 'Geist', sans-serif; font-weight: 600; cursor: pointer; }
        .es-btn--save { background: var(--accent); color: var(--accent-fg); }
        .es-btn--save:disabled { opacity: 0.4; cursor: not-allowed; }
        .es-btn--delete { background: #cc0000; color: #fff; }
        .es-slider-wrap { padding: 4px 0 11px; border-bottom: 1px solid var(--border-2); margin-bottom: 10px; }
        .es-slider-label { display: flex; justify-content: space-between; font-size: 14px; color: var(--text); margin-bottom: 8px; }
        .es-slider { width: 100%; -webkit-appearance: none; appearance: none; height: 4px; border-radius: 2px; outline: none; cursor: pointer; background: linear-gradient(to right, var(--accent) calc(var(--v) * 1%), var(--border) calc(var(--v) * 1%)); }
        .es-slider::-webkit-slider-thumb { -webkit-appearance: none; width: 18px; height: 18px; border-radius: 50%; background: var(--surface); border: 2px solid var(--accent); cursor: pointer; }
        .es-pw-wrap { position: relative; display: flex; align-items: center; margin-bottom: 10px; }
        .es-pw-toggle { position: absolute; right: 10px; background: none; border: none; cursor: pointer; color: var(--text-4); padding: 0; display: flex; align-items: center; }
      `}</style>
      <div className="es-overlay">
        <div className="es-modal" onClick={e => e.stopPropagation()}>
          <div className="es-header">
            <IconCircleHalf2 size={22} />
            <span className="es-title">Редактирование сервера</span>
            <button type="button" className="es-close" onClick={onClose} aria-label="Закрыть"><IconX size={18} /></button>
          </div>

          <input className="es-input" placeholder="Название сервера" value={name} onChange={e => setName(e.target.value)} />
          <div style={{ display: 'flex', gap: 8 }}>
            <input className="es-input" style={{ flex: 1 }} placeholder="IP сервера" value={serverIp} onChange={e => setServerIp(e.target.value)} />
            <input className="es-input" style={{ width: 100 }} placeholder="Порт" value={serverPort} onChange={e => setServerPort(e.target.value)} />
          </div>
          <div className="es-pw-wrap">
            <input className="es-input" placeholder="Пароль туннеля" type={showPassword ? 'text' : 'password'} value={password} onChange={e => setPassword(e.target.value)} style={{ paddingRight: 36, marginBottom: 0 }} />
            <button type="button" className="es-pw-toggle" onClick={() => setShowPassword(v => !v)} aria-label={showPassword ? 'Скрыть пароль' : 'Показать пароль'}>
              {showPassword ? <IconEyeOff size={18} /> : <IconEye size={18} />}
            </button>
          </div>

          <div className="es-slider-wrap">
            <div className="es-slider-label">
              <span>Мощность</span>
              <span>{filledHashes === 0 ? 'нет хешей' : power}</span>
            </div>
            <input
              type="range" min={9} max={powerMax} step={9}
              value={Math.min(power, powerMax)}
              className="es-slider"
              disabled={filledHashes === 0}
              style={{ '--v': filledHashes > 0 ? Math.round((Math.min(power, powerMax) - 9) / Math.max(powerMax - 9, 1) * 100) : 0 } as React.CSSProperties}
              onChange={e => setPower(+e.target.value)}
              aria-label="Мощность"
            />
          </div>

          <button
            type="button"
            className="es-hash-btn"
            onClick={() => setHashOpen(true)}
          >
            <IconHash stroke={2} size={16} />
            Хеши ({filledHashes}/4)
          </button>

          <div className="es-btn-row">
            <button type="button" className="es-btn es-btn--save" onClick={handleSave} disabled={!name.trim() || !serverIp.trim() || saving}>
              {saving ? 'Сохранение...' : 'Сохранить'}
            </button>
            <button type="button" className="es-btn es-btn--delete" onClick={handleDelete}>Удалить</button>
          </div>
        </div>
      </div>
      {hashOpen && (
        <Hash
          hashes={hashes}
          onClose={() => setHashOpen(false)}
          onSave={handleHashSave}
        />
      )}
    </>
  );
}
