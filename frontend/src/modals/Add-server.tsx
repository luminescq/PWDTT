import { useState } from 'react';
import { IconCircleHalf2, IconEye, IconEyeOff, IconX, IconHash } from '@tabler/icons-react';
import type { Server } from '../lib/types';
import { SaveProfile } from '../../wailsjs/go/backend/App';
import { parseWdttUrl } from '../lib/utils/wdttLink';
import { toastStore } from '../lib/stores/toastStore';
import Hash from './Hash';

interface Props {
  onClose: () => void;
  onAdd: (server: Omit<Server, 'id'>) => void;
}

export default function AddServer({ onClose, onAdd }: Props) {
  const [link, setLink] = useState('');
  const [name, setName] = useState('');
  const [ip, setIp] = useState('');
  const [port, setPort] = useState('56000');
  const [password, setPassword] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const [hashes, setHashes] = useState<[string,string,string,string]>(['', '', '', '']);
  const [hashOpen, setHashOpen] = useState(false);

  const filledHashes = hashes.filter(h => h.trim()).length;
  const powerMax = Math.max(9, filledHashes * 27);
  const [power, setPower] = useState(9);

  const applyLink = (raw: string) => {
    setLink(raw);
    const parsed = parseWdttUrl(raw.trim());
    if (!parsed) return;
    // host может быть "IP:PORT" — разделяем
    const lastColon = parsed.host.lastIndexOf(':');
    if (lastColon > 0 && lastColon < parsed.host.length - 1) {
      setIp(parsed.host.slice(0, lastColon));
      setPort(parsed.host.slice(lastColon + 1));
    } else {
      setIp(parsed.host);
    }
    setPassword(parsed.password);
    if (parsed.name !== 'Server') setName(parsed.name);
    if (parsed.hashes.length > 0) {
      const h4: [string,string,string,string] = [parsed.hashes[0]??'', parsed.hashes[1]??'', parsed.hashes[2]??'', parsed.hashes[3]??''];
      setHashes(h4);
      const filled = h4.filter(x => x.trim()).length;
      setPower(Math.max(9, filled * 9));
    }
  };

  const handleHashSave = (h: [string, string, string, string]) => {
    setHashes(h);
    const filled = h.filter(x => x.trim()).length;
    const newMax = Math.max(9, filled * 27);
    setPower(p => Math.min(p, newMax) || Math.max(9, filled * 9));
  };

  const [saving, setSaving] = useState(false);

  const handleAdd = async () => {
    if (!name.trim() || !ip.trim() || saving) return;
    setSaving(true);
    const host = `${ip.trim()}:${port.trim() || '56000'}`;

    try {
      await SaveProfile(name.trim(), {
        peer: host,
        password,
        hashes: hashes.filter(h => h.trim()),
        turn: '',
        port: '',
        device_id: '',
        listen: '127.0.0.1:9000',
        turn_tcp: false,
      });
    } catch (e) {
      console.warn('SaveProfile failed:', e);
      toastStore.show('Не удалось сохранить профиль на диск', 3000);
      setSaving(false);
      return; // профиль в бэкенде — источник истины; без него сервер "потеряет" хеши
    }

    onAdd({ name: name.trim(), host, password, hashes, power });
    setSaving(false);
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
        .as-slider-wrap { padding: 4px 0 11px; border-bottom: 1px solid var(--border-2); margin-bottom: 10px; }
        .as-slider-label { display: flex; justify-content: space-between; font-size: 14px; color: var(--text); margin-bottom: 8px; }
        .as-slider { width: 100%; -webkit-appearance: none; appearance: none; height: 4px; border-radius: 2px; outline: none; cursor: pointer; background: linear-gradient(to right, var(--accent) calc(var(--v) * 1%), var(--border) calc(var(--v) * 1%)); }
        .as-slider::-webkit-slider-thumb { -webkit-appearance: none; width: 18px; height: 18px; border-radius: 50%; background: var(--surface); border: 2px solid var(--accent); cursor: pointer; }
        .as-hash-btn { width: 100%; margin-top: 4px; margin-bottom: 10px; padding: 13px; border: 1.5px solid var(--border); border-radius: 10px; background: var(--surface); color: var(--text); font-size: 14px; font-family: 'Geist', sans-serif; font-weight: 600; cursor: pointer; display: flex; align-items: center; justify-content: center; gap: 8px; transition: border-color 0.15s, box-shadow 0.15s; }
        .as-hash-btn:hover { border-color: var(--accent); box-shadow: 0 0 16px var(--accent-dim, rgba(139,92,246,0.2)); }
        .as-pw-wrap { position: relative; display: flex; align-items: center; margin-bottom: 10px; }
        .as-pw-toggle { position: absolute; right: 10px; background: none; border: none; cursor: pointer; color: var(--text-4); padding: 0; display: flex; align-items: center; }
      `}</style>
      <div className="as-overlay">
        <div className="as-modal" onClick={e => e.stopPropagation()}>
          <div className="as-header">
            <IconCircleHalf2 stroke={2} size={22} />
            <span className="as-title">Добавление сервера</span>
            <button type="button" className="as-close" onClick={onClose} aria-label="Закрыть"><IconX size={18} /></button>
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
          <div className="as-pw-wrap">
            <input className="as-input" placeholder="Пароль туннеля" type={showPassword ? 'text' : 'password'} value={password} onChange={e => setPassword(e.target.value)} style={{ paddingRight: 36, marginBottom: 0 }} />
            <button type="button" className="as-pw-toggle" onClick={() => setShowPassword(v => !v)} aria-label={showPassword ? 'Скрыть пароль' : 'Показать пароль'}>
              {showPassword ? <IconEyeOff size={18} /> : <IconEye size={18} />}
            </button>
          </div>

          <div className="as-slider-wrap">
            <div className="as-slider-label">
              <span>Мощность</span>
              <span>{filledHashes === 0 ? 'нет хешей' : power}</span>
            </div>
            <input
              type="range" min={9} max={powerMax} step={9}
              value={Math.min(power, powerMax)}
              className="as-slider"
              disabled={filledHashes === 0}
              style={{ '--v': filledHashes > 0 ? Math.round((Math.min(power, powerMax) - 9) / Math.max(powerMax - 9, 1) * 100) : 0 } as React.CSSProperties}
              onChange={e => setPower(+e.target.value)}
              aria-label="Мощность"
            />
          </div>

          <button type="button" className="as-hash-btn" onClick={() => setHashOpen(true)}>
            <IconHash stroke={2} size={16} />
            Хеши ({filledHashes}/4)
          </button>

          <button type="button" className="as-btn" onClick={handleAdd} disabled={!name.trim() || !ip.trim() || saving}>
            {saving ? 'Сохранение...' : 'Добавить сервер'}
          </button>
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
