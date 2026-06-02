import { useState, useEffect, useRef } from 'react';
import { IconLockPassword, IconServer2, IconServerOff } from '@tabler/icons-react';
import Secrets from './Secrets';
import { deployStore } from '../lib/store';
import type { DeployConfig, DeployState } from '../lib/types';
import { Deploy as WailsDeploy, Undeploy as WailsUndeploy } from '../../wailsjs/go/backend/App';
import { EventsOn } from '../../wailsjs/runtime/runtime';

interface Props {
  onClose: () => void;
}

export default function Deploy({ onClose }: Props) {
  const [cfg, setCfg] = useState<DeployConfig>(() => deployStore.get());
  const [secretsOpen, setSecretsOpen] = useState(false);
  const [deployState, setDeployState] = useState<DeployState>('idle');
  const [logs, setLogs] = useState<string[]>([]);
  const logRef = useRef<HTMLDivElement>(null);

  const set = <K extends keyof DeployConfig>(k: K, v: DeployConfig[K]) => {
    const next = { ...cfg, [k]: v };
    setCfg(next);
    deployStore.save(next);
  };

  const handleSecretsSave = (partial: Partial<DeployConfig>) => {
    const next = { ...cfg, ...partial };
    setCfg(next);
    deployStore.save(next);
  };

  useEffect(() => {
    const offLog = EventsOn('deploy_log', (msg: string) => {
      setLogs(prev => [...prev, msg]);
    });
    const offDone = EventsOn('deploy_done', () => setDeployState('idle'));
    return () => { offLog(); offDone(); };
  }, []);

  useEffect(() => {
    logRef.current?.scrollTo(0, logRef.current.scrollHeight);
  }, [logs]);

  const buildParams = () => ({
    host: cfg.host.trim(),
    login: cfg.login.trim() || 'root',
    password: cfg.password,
    sshPort: cfg.sshPort || '22',
    mainPassword: cfg.tunnelPassword,
    adminId: cfg.tgAdminId,
    botToken: cfg.tgBotToken,
    dtlsPort: cfg.portsManual ? parseInt(cfg.dtlsPort) || 56000 : 56000,
    wgPort: cfg.portsManual ? parseInt(cfg.wgPort) || 56001 : 56001,
  });

  const canDeploy = cfg.host.trim() && cfg.password.trim() && cfg.tunnelPassword.trim();
  const busy = deployState !== 'idle';

  const handleInstall = async () => {
    if (!canDeploy || busy) return;
    setLogs([]);
    setDeployState('deploying');
    try {
      await WailsDeploy(buildParams());
    } catch (e: any) {
      setLogs(prev => [...prev, '❌ ' + String(e)]);
      setDeployState('idle');
    }
  };

  const handleRemove = async () => {
    if (!cfg.host.trim() || !cfg.password.trim() || busy) return;
    setLogs([]);
    setDeployState('removing');
    try {
      await WailsUndeploy(buildParams());
    } catch (e: any) {
      setLogs(prev => [...prev, '❌ ' + String(e)]);
      setDeployState('idle');
    }
  };

  return (
    <>
      <style>{`
        .modal-overlay { position: fixed; inset: 0; background: var(--overlay-bg); backdrop-filter: blur(4px); display: flex; align-items: center; justify-content: center; z-index: 100; animation: overlay-in 0.3s ease-out; }
        .modal { background: var(--surface); border-radius: 14px; padding: 20px; width: 380px; max-width: 95vw; box-shadow: var(--shadow); border: 1px solid var(--border); animation: modal-in 0.3s ease-out; }
        .modal-header { display: flex; align-items: center; gap: 10px; margin-bottom: 18px; color: var(--text); }
        .modal-title { font-size: 16px; font-weight: 600; flex: 1; color: var(--text); }
        .modal-close { background: none; border: none; cursor: pointer; font-size: 18px; color: var(--text); line-height: 1; padding: 0; }
        .modal-input { width: 100%; padding: 11px 14px; border: 1.5px solid var(--input-border); border-radius: 10px; font-size: 14px; font-family: 'Geist', sans-serif; outline: none; margin-bottom: 10px; box-sizing: border-box; color: var(--text); background: var(--input-bg); }
        .modal-input::placeholder { color: var(--text-4); }
        .modal-btn-row { display: flex; gap: 10px; margin-top: 16px; }
        .modal-btn { flex: 1; padding: 13px; border: none; border-radius: 10px; font-size: 14px; font-family: 'Geist', sans-serif; font-weight: 600; cursor: pointer; display: flex; align-items: center; justify-content: center; gap: 8px; transition: opacity 0.2s; }
        .modal-btn:disabled { opacity: 0.45; cursor: not-allowed; }
        .modal-btn--black { background: var(--accent); color: var(--accent-fg); }
        .modal-btn--red { background: #cc0000; color: #fff; }
        .secrets-btn { width: 100%; padding: 11px 14px; border: 1.5px solid var(--border); border-radius: 10px; background: var(--surface); color: var(--text); font-size: 14px; font-family: 'Geist', sans-serif; font-weight: 500; cursor: pointer; display: flex; align-items: center; justify-content: center; gap: 8px; margin-bottom: 10px; }
        .ports-row { display: flex; align-items: center; justify-content: space-between; padding: 4px 0; margin-bottom: 10px; font-size: 14px; color: var(--text); }
        .toggle { width: 48px; height: 26px; border-radius: 50px; border: 1.5px solid var(--input-border); background: var(--surface); cursor: pointer; position: relative; transition: background 0.2s; flex-shrink: 0; }
        .toggle::after { content: ''; position: absolute; width: 18px; height: 18px; border-radius: 50%; background: var(--text-3); top: 2px; left: 2px; transition: left 0.2s; }
        .toggle--on { background: var(--accent); border-color: var(--accent); }
        .toggle--on::after { background: var(--accent-fg); left: 24px; }
        .deploy-log { background: #0d0d0f; color: #a1a1aa; border-radius: 8px; padding: 10px; font-size: 12px; font-family: monospace; max-height: 140px; overflow-y: auto; margin-top: 14px; white-space: pre-wrap; word-break: break-all; }
      `}</style>
      <div className="modal-overlay" onClick={onClose}>
        <div className="modal" onClick={e => e.stopPropagation()}>
          <div className="modal-header">
            <IconServer2 stroke={2} size={20} />
            <span className="modal-title">Деплой</span>
            <button className="modal-close" onClick={onClose}>✕</button>
          </div>

          <input className="modal-input" placeholder="IP или Домен (без порта)" value={cfg.host} onChange={e => set('host', e.target.value)} disabled={busy} />
          <input className="modal-input" placeholder="Логин (по умолчанию root)" value={cfg.login} onChange={e => set('login', e.target.value)} disabled={busy} />
          <input className="modal-input" placeholder="Пароль SSH" type="password" value={cfg.password} onChange={e => set('password', e.target.value)} disabled={busy} />

          <button className="secrets-btn" onClick={() => setSecretsOpen(true)} disabled={busy}>
            <IconLockPassword stroke={2} size={18} />
            <span>Секреты{cfg.tunnelPassword ? ' ✓' : ' — не заданы'}</span>
          </button>

          <div className="ports-row">
            <span>Ручное управление портами</span>
            <button className={`toggle${cfg.portsManual ? ' toggle--on' : ''}`} onClick={() => set('portsManual', !cfg.portsManual)} disabled={busy} />
          </div>

          <div className="modal-btn-row">
            <button className="modal-btn modal-btn--black" onClick={handleInstall} disabled={!canDeploy || busy}>
              <IconServer2 size={18} />
              {deployState === 'deploying' ? 'Установка...' : 'Установить'}
            </button>
            <button className="modal-btn modal-btn--red" onClick={handleRemove} disabled={!cfg.host.trim() || !cfg.password.trim() || busy}>
              <IconServerOff size={18} />
              {deployState === 'removing' ? 'Удаление...' : 'Удалить'}
            </button>
          </div>

          {logs.length > 0 && (
            <div className="deploy-log" ref={logRef}>
              {logs.join('\n')}
            </div>
          )}
        </div>
      </div>

      {secretsOpen && (
        <Secrets
          onClose={() => setSecretsOpen(false)}
          onSave={handleSecretsSave}
          showPorts={cfg.portsManual}
        />
      )}
    </>
  );
}
