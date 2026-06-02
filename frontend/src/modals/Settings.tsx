import { useState, useEffect } from 'react';
import { IconSettings2, IconHash } from '@tabler/icons-react';
import Hash from './Hash';
import { settingsStore } from '../lib/store';
import { tunnelStore } from '../lib/stores/tunnelStore';
import type { AppSettings } from '../lib/types';
import { SetTrayEnabled, SetAutoStart, GetAutoStart } from '../../wailsjs/go/backend/App';

interface Props {
  onClose: () => void;
}

export default function Settings({ onClose }: Props) {
  const [settings, setSettings] = useState<AppSettings>(() => settingsStore.get());
  const [hashOpen, setHashOpen] = useState(false);
  const [mtuRaw, setMtuRaw] = useState(String(settingsStore.get().mtu ?? 1280));
  const mtuValid = (() => { const n = Number(mtuRaw); return Number.isInteger(n) && n >= 576 && n <= 1500; })();
  const [tunnelState, setTunnelState] = useState(() => tunnelStore.get());
  useEffect(() => tunnelStore.subscribe(setTunnelState), []);
  const locked = tunnelState === 'connected' || tunnelState === 'connecting';

  // Sync autoStart from backend on open
  useEffect(() => {
    GetAutoStart().then(v => {
      if (v !== settings.autoStart) update('autoStart', v);
    });
  }, []);

  const update = <K extends keyof AppSettings>(key: K, value: AppSettings[K]) => {
    setSettings(s => ({ ...s, [key]: value }));
  };

  const handleClose = () => {
    const n = Number(mtuRaw);
    const mtu = mtuValid ? n : settings.mtu;
    settingsStore.save({ ...settings, mtu });
    onClose();
  };

  const filledHashes = settings.hashes.filter(h => h.trim()).length;
  const powerMax = Math.max(9, filledHashes * 27);

  return (
    <>
      <style>{`
        .st-overlay { position: fixed; inset: 0; background: var(--overlay-bg); backdrop-filter: blur(4px); display: flex; align-items: center; justify-content: center; z-index: 100; animation: overlay-in 0.3s ease-out; }
        .st-modal { background: var(--surface); border-radius: 14px; padding: 20px; width: 380px; max-width: 95vw; box-shadow: var(--shadow); animation: modal-in 0.3s ease-out; border: 1px solid var(--border); }
        .st-header { display: flex; align-items: center; gap: 10px; margin-bottom: 18px; color: var(--text); }
        .st-title { font-size: 16px; font-weight: 600; flex: 1; color: var(--text); }
        .st-close { background: none; border: none; cursor: pointer; font-size: 18px; color: var(--text); line-height: 1; padding: 0; }
        .st-row { display: flex; align-items: center; justify-content: space-between; padding: 11px 0; border-bottom: 1px solid var(--border-2); font-size: 14px; color: var(--text); }
        .st-row:last-of-type { border-bottom: none; }
        .st-toggle { width: 48px; height: 26px; border-radius: 50px; border: none; cursor: pointer; position: relative; transition: background 0.2s; flex-shrink: 0; }
        .st-toggle--on { background: var(--accent); }
        .st-toggle--off { background: var(--seg-bg); }
        .st-toggle::after { content: ''; position: absolute; width: 18px; height: 18px; border-radius: 50%; top: 4px; transition: left 0.2s; }
        .st-toggle--on::after { background: var(--accent-fg); left: 26px; }
        .st-toggle--off::after { background: var(--text-3); left: 4px; }
        .st-seg { display: flex; background: var(--seg-bg); border-radius: 8px; padding: 2px; gap: 2px; }
        .st-seg-btn { padding: 5px 13px; border: none; border-radius: 6px; font-size: 12px; font-weight: 600; cursor: pointer; transition: background 0.15s, color 0.15s; background: transparent; color: var(--seg-text); }
        .st-seg-btn--active { background: var(--accent); color: var(--accent-fg); }
        .st-slider-wrap { padding: 4px 0 11px; border-bottom: 1px solid var(--border-2); }
        .st-slider-label { display: flex; justify-content: space-between; font-size: 14px; color: var(--text); margin-bottom: 8px; }
        .st-slider { width: 100%; -webkit-appearance: none; appearance: none; height: 4px; border-radius: 2px; outline: none; cursor: pointer; background: linear-gradient(to right, var(--accent) calc(var(--v) * 1%), var(--border) calc(var(--v) * 1%)); }
        .st-slider::-webkit-slider-thumb { -webkit-appearance: none; width: 18px; height: 18px; border-radius: 50%; background: var(--surface); border: 2px solid var(--accent); cursor: pointer; }
        .st-num-input { width: 80px; padding: 5px 10px; border: 1.5px solid var(--border); border-radius: 8px; font-size: 14px; font-family: 'Geist', sans-serif; text-align: right; outline: none; background: var(--input-bg); color: var(--text); transition: border-color 0.15s; }
        .st-num-input:focus { border-color: var(--accent); }
        .st-num-input--error { border-color: #ef4444; }
        .st-hash-btn { width: 100%; margin-top: 16px; padding: 13px; border: 1.5px solid var(--border); border-radius: 10px; background: var(--surface); color: var(--text); font-size: 14px; font-family: 'Geist', sans-serif; font-weight: 600; cursor: pointer; display: flex; align-items: center; justify-content: center; gap: 8px; }
        .st-locked { opacity: 0.4; pointer-events: none; }
        .st-lock-hint { font-size: 11px; color: var(--text-3); margin-bottom: 4px; text-align: center; }
      `}</style>
      <div className="st-overlay" onClick={handleClose}>
        <div className="st-modal" onClick={e => e.stopPropagation()}>
          <div className="st-header">
            <IconSettings2 stroke={2} size={20} />
            <span className="st-title">Настройки</span>
            <button className="st-close" onClick={handleClose}>✕</button>
          </div>

          {locked && <div className="st-lock-hint">Недоступно во время подключения</div>}

          <div className={`st-slider-wrap${locked ? ' st-locked' : ''}`}>
            <div className="st-slider-label"><span>Мощность</span><span>{settings.power}</span></div>
            <input
              type="range" min={9} max={powerMax} step={9} value={Math.min(settings.power, powerMax)}
              className="st-slider"
              style={{ '--v': Math.round((Math.min(settings.power, powerMax) - 9) / Math.max(powerMax - 9, 1) * 100) } as React.CSSProperties}
              onChange={e => update('power', +e.target.value)}
            />
          </div>

          <div className={`st-row${locked ? ' st-locked' : ''}`}>
            <span>MTU</span>
            <input
              type="number" min={576} max={1500} step={1}
              value={mtuRaw}
              className={`st-num-input${!mtuValid ? ' st-num-input--error' : ''}`}
              onChange={e => setMtuRaw(e.target.value)}
              onBlur={() => {
                const n = Number(mtuRaw);
                const clamped = Number.isFinite(n) ? Math.max(576, Math.min(1500, Math.round(n))) : 1280;
                setMtuRaw(String(clamped));
                update('mtu', clamped);
              }}
            />
          </div>

          <div className="st-row">
            <span>Трей</span>
            <button className={`st-toggle st-toggle--${settings.tray ? 'on' : 'off'}`} onClick={() => {
              const next = !settings.tray;
              update('tray', next);
              SetTrayEnabled(next);
            }} />
          </div>

          <div className="st-row">
            <span>Запускать при старте</span>
            <button className={`st-toggle st-toggle--${settings.autoStart ? 'on' : 'off'}`} onClick={() => {
              const next = !settings.autoStart;
              update('autoStart', next);
              SetAutoStart(next);
            }} />
          </div>

          <button className="st-hash-btn" onClick={() => setHashOpen(true)}>
            <IconHash stroke={2} size={16} />
            VK Хеши ({filledHashes}/4)
          </button>
        </div>
      </div>
      {hashOpen && (
        <Hash
          hashes={settings.hashes}
          onClose={() => setHashOpen(false)}
          onSave={hashes => update('hashes', hashes)}
        />
      )}
    </>
  );
}
