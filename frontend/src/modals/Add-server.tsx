import { useState } from 'react';
import { IconCircleHalf2, IconEye, IconEyeOff, IconX } from '@tabler/icons-react';
import type { Server } from '../lib/types';
import { SaveProfile, Encrypt } from '../../wailsjs/go/backend/App';
import { parseWdttUrl } from '../lib/utils/wdttLink';

interface Props {
  onClose: () => void;
  onAdd: (server: Omit<Server, 'id'>) => Promise<void>;
}

export default function AddServer({ onClose, onAdd }: Props) {
  const [link, setLink] = useState('');
  const [name, setName] = useState('');
  const [ip, setIp] = useState('');
  const [port, setPort] = useState('56000');
  const [password, setPassword] = useState('');
  const [showPassword, setShowPassword] = useState(false);

  const applyLink = (raw: string) => {
    setLink(raw);
    const parsed = parseWdttUrl(raw.trim());
    if (!parsed) return;
    setIp(parsed.ip);
    setPort(parsed.dtlsPort);
    setPassword(parsed.password);
    if (parsed.name !== 'Server') setName(parsed.name);
  };

  const handleAdd = async () => {
    if (!name.trim() || !ip.trim()) return;
    const host = `${ip.trim()}:${port.trim() || '56000'}`;
    const parsed = parseWdttUrl(link.trim());
    const hashes = parsed?.hashes ?? [];

    try {
      await SaveProfile(name.trim(), {
        peer: host,
        password,
        hashes,
        turn: '', port: '', device_id: '', listen: '',
      });
    } catch (e) {
      console.warn('SaveProfile failed:', e);
    }

    const h4: [string,string,string,string] = [hashes[0]??'', hashes[1]??'', hashes[2]??'', hashes[3]??''];
    await onAdd({ name: name.trim(), host, password, hashes: h4 });
    onClose();
  };

  return (
    <>
      <style>{`
        .as-overlay { position: fixed; inset: 0; background: var(--overlay-bg); backdrop-filter: blur(4px); display: flex; align-items: center; justify-content: center; z-index: 100; animation: overlay-in 0.3s ease-out; }
        .as-modal { background: var(--surface); border-radius: 14px; padding: 20px; width: 380px; max-width: 95vw; box-shadow: var(--shadow); border: 1px solid var(--border); max-height: 90vh; overflow-y: auto; animation: modal-in 0.3s ease-out; }
        .as-header { display: flex; align-items: center; gap: 10px; margin-bottom: 18px; color: var(--text); }
        .as-title { font-size: 16px; font-weight: 600; flex: 1; color: var(--text); }
        .as-close { background: none; border: none; cursor: pointer; font-size: 18px; color: var(--text); line-height: 1; padding: 0; }
        .as-input { width: 100%; padding: 11px 14px; border: 1.5px solid var(--input-border); border-radius: 10px; font-size: 14px; font-family: 'Geist', sans-serif; outline: none; margin-bottom: 10px; box-sizing: border-box; color: var(--text); background: var(--input-bg); }
        .as-input::placeholder { color: var(--text-4); }
        .as-divider { display: flex; align-items: center; gap: 8px; margin: 4px 0 12px; color: var(--text-4); font-size: 12px; }
        .as-divider::before, .as-divider::after { content: ''; flex: 1; height: 1px; background: var(--border); }
        .as-btn { width: 100%; padding: 13px; border: none; border-radius: 10px; background: var(--accent); color: var(--accent-fg); font-size: 14px; font-family: 'Geist', sans-serif; font-weight: 600; cursor: pointer; margin-top: 4px; }
        .as-btn:disabled { opacity: 0.4; cursor: not-allowed; }
      `}</style>
      <div className="as-overlay">
        <div className="as-modal" onClick={e => e.stopPropagation()}>
          <div className="as-header">
            <IconCircleHalf2 stroke={2} size={22} />
            <span className="as-title">Добавление сервера</span>
            <button className="as-close" onClick={onClose}><IconX size={18} /></button>
          </div>

          <input
            className="as-input"
            placeholder="Вставьте ссылку wdtt://..."
            value={link}
            onChange={e => applyLink(e.target.value)}
          />

          <div className="as-divider">или вручную</div>

          <input className="as-input" placeholder="Название сервера" value={name} onChange={e => setName(e.target.value)} />
          <div style={{ display: 'flex', gap: 8 }}>
            <input className="as-input" style={{ flex: 1 }} placeholder="IP сервера" value={ip} onChange={e => setIp(e.target.value)} />
            <input className="as-input" style={{ width: 100 }} placeholder="Порт" value={port} onChange={e => setPort(e.target.value)} />
          </div>
          <div style={{ position: 'relative', display: 'flex', alignItems: 'center', marginBottom: 10 }}>
            <input className="as-input" placeholder="Пароль туннеля" type={showPassword ? 'text' : 'password'} value={password} onChange={e => setPassword(e.target.value)} style={{ paddingRight: 36, marginBottom: 0 }} />
            <button type="button" onClick={() => setShowPassword(v => !v)} style={{ position: 'absolute', right: 10, background: 'none', border: 'none', cursor: 'pointer', color: 'var(--text-4)', padding: 0, display: 'flex', alignItems: 'center' }}>
              {showPassword ? <IconEyeOff size={18} /> : <IconEye size={18} />}
            </button>
          </div>
          <button className="as-btn" onClick={handleAdd} disabled={!name.trim() || !ip.trim()}>Добавить сервер</button>
        </div>
      </div>
    </>
  );
}
