import { useState } from 'react';
import { IconCodeAsterisk, IconRestore } from '@tabler/icons-react';
import { deployStore } from '../lib/store';
import type { DeployConfig } from '../lib/types';

interface Props {
  onClose: () => void;
  onSave: (cfg: Partial<DeployConfig>) => void;
  showPorts: boolean;
}

export default function Secrets({ onClose, onSave, showPorts }: Props) {
  const [cfg, setCfg] = useState<DeployConfig>(() => deployStore.get());

  const set = <K extends keyof DeployConfig>(k: K, v: DeployConfig[K]) =>
    setCfg(c => ({ ...c, [k]: v }));

  const generatePassword = () => {
    const chars = 'ABCDEFGHJKLMNPQRSTUVWXYZabcdefghjkmnpqrstuvwxyz23456789';
    set('tunnelPassword', Array.from({ length: 16 }, () => chars[Math.floor(Math.random() * chars.length)]).join(''));
  };

  const handleSave = () => {
    deployStore.save(cfg);
    onSave(cfg);
    onClose();
  };

  return (
    <>
      <style>{`
        .modal-overlay { position: fixed; inset: 0; background: var(--overlay-bg); backdrop-filter: blur(4px); display: flex; align-items: center; justify-content: center; z-index: 200; animation: overlay-in 0.3s ease-out; }
        .modal { background: var(--surface); border-radius: 14px; padding: 20px; width: 380px; max-width: 95vw; box-shadow: var(--shadow); border: 1px solid var(--border); animation: modal-in 0.3s ease-out; }
        .modal-header { display: flex; align-items: center; gap: 10px; margin-bottom: 18px; color: var(--text); }
        .modal-title { font-size: 16px; font-weight: 600; flex: 1; color: var(--text); }
        .modal-close { background: none; border: none; cursor: pointer; font-size: 18px; color: var(--text); line-height: 1; padding: 0; }
        .modal-input { width: 100%; padding: 11px 14px; border: 1.5px solid var(--input-border); border-radius: 10px; font-size: 14px; font-family: 'Geist', sans-serif; outline: none; margin-bottom: 10px; box-sizing: border-box; color: var(--text); background: var(--input-bg); }
        .modal-input::placeholder { color: var(--text-4); }
        .modal-section-label { display: flex; align-items: center; gap: 10px; font-size: 12px; color: var(--text-3); margin: 6px 0 10px; white-space: nowrap; }
        .modal-section-label::before, .modal-section-label::after { content: ''; flex: 1; height: 1px; background: var(--border); }
        .modal-btn { width: 100%; padding: 13px; border: none; border-radius: 10px; background: var(--accent); color: var(--accent-fg); font-size: 14px; font-family: 'Geist', sans-serif; font-weight: 600; cursor: pointer; margin-top: 16px; }
      `}</style>
      <div className="modal-overlay" onClick={onClose}>
        <div className="modal" onClick={e => e.stopPropagation()}>
          <div className="modal-header">
            <IconCodeAsterisk stroke={2} size={20} />
            <span className="modal-title">Секреты</span>
            <button className="modal-close" onClick={onClose}>✕</button>
          </div>

          <div style={{ position: 'relative', marginBottom: 12 }}>
            <input
              className="modal-input"
              placeholder="Пароль туннеля"
              style={{ paddingRight: 44, marginBottom: 0 }}
              value={cfg.tunnelPassword}
              onChange={e => set('tunnelPassword', e.target.value)}
            />
            <button
              style={{ position: 'absolute', right: 14, top: '50%', transform: 'translateY(-50%)', background: 'none', border: 'none', cursor: 'pointer', color: 'var(--text)' }}
              onClick={generatePassword}
              title="Сгенерировать"
            >
              <IconRestore size={18} />
            </button>
          </div>

          <div className="modal-section-label">Телеграм бот (опционально)</div>
          <input className="modal-input" placeholder="ID Админа" value={cfg.tgAdminId} onChange={e => set('tgAdminId', e.target.value)} />
          <input className="modal-input" placeholder="Токен Бота" value={cfg.tgBotToken} onChange={e => set('tgBotToken', e.target.value)} />

          <div className="modal-section-label">SSH Порт</div>
          <input className="modal-input" placeholder="Порт для деплоя SSH" value={cfg.sshPort} onChange={e => set('sshPort', e.target.value)} />

          {showPorts && <>
            <div className="modal-section-label">Порты сервера</div>
            <input className="modal-input" placeholder="Порт DTLS сервера" value={cfg.dtlsPort} onChange={e => set('dtlsPort', e.target.value)} />
            <input className="modal-input" placeholder="Порт WireGuard сервера" value={cfg.wgPort} onChange={e => set('wgPort', e.target.value)} />
          </>}

          <button className="modal-btn" onClick={handleSave}>Сохранить</button>
        </div>
      </div>
    </>
  );
}
