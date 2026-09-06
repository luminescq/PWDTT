import { useState, useEffect, useRef, useCallback } from 'react';
import { IconSettings2, IconX, IconHeartFilled, IconBug, IconAlertTriangle } from '@tabler/icons-react';
import { settingsStore } from '../lib/store';
import type { AppSettings } from '../lib/types';
import { SetAutoStart, GetAutoStart, GetVersion, GenerateReport, GetObfsAccepted, SetObfsAccepted, SetObfsMode } from '../../wailsjs/go/backend/App';
import { BrowserOpenURL } from '../../wailsjs/runtime/runtime';
import { logStore } from '../lib/stores/logStore';
import { toastStore } from '../lib/stores/toastStore';
import './Settings.css';

interface Props {
  onClose: () => void;
}

export default function Settings({ onClose }: Props) {
  const [settings, setSettings] = useState<AppSettings>(() => settingsStore.get());
  const [version, setVersion] = useState('...');
  const copiedTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const [showObfsModal, setShowObfsModal] = useState(false);
  const [pendingObfsMode, setPendingObfsMode] = useState<'audio' | 'video'>('audio');

  const update = useCallback(<K extends keyof AppSettings>(key: K, value: AppSettings[K]) => {
    setSettings(s => {
      const next = { ...s, [key]: value };
      settingsStore.save(next);
      return next;
    });
  }, []);

  useEffect(() => {
    return () => { if (copiedTimerRef.current) clearTimeout(copiedTimerRef.current); };
  }, []);

  useEffect(() => {
    GetAutoStart().then(v => {
      if (v !== settings.autoStart) update('autoStart', v);
    }).catch(() => { toastStore.show('Не удалось загрузить настройки', 3000); });
    GetObfsAccepted().then(v => {
      update('obfsAccepted', v);
    }).catch(() => {});
    GetVersion().then(setVersion).catch(() => {});
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const handleReport = async () => {
    const logs = logStore.getAll();
    const report = await GenerateReport(logs.map(e => ({
      level: e.level,
      message: e.message,
      time: e.time,
      count: e.count,
    })));
    const mode = settings.obfsMode || 'audio';
    const accepted = settings.obfsAccepted ? 'да' : 'нет';
    const full = report + `\n## Settings\n- Obfuscation: ${mode}\n- Obfs accepted: ${accepted}\n`;
    await navigator.clipboard.writeText(full);
    setCopiedReport(true);
    if (copiedTimerRef.current) clearTimeout(copiedTimerRef.current);
    copiedTimerRef.current = setTimeout(() => setCopiedReport(false), 2000);
  };

  const [copiedReport, setCopiedReport] = useState(false);

  const handleObfsClick = async (mode: 'audio' | 'video') => {
    if (settings.obfsAccepted) {
      update('obfsMode', mode);
      await SetObfsMode(mode);
    } else {
      setPendingObfsMode(mode);
      setShowObfsModal(true);
    }
  };

  const handleObfsAccept = async () => {
    try {
      await SetObfsAccepted(true);
      await SetObfsMode(pendingObfsMode);
      update('obfsAccepted', true);
      update('obfsMode', pendingObfsMode);
      setShowObfsModal(false);
    } catch (e) {
      toastStore.show('Не удалось сохранить настройки', 3000);
    }
  };

  return (
    <>
      <div className="st-overlay" onClick={onClose}>
        <div className="st-modal" onClick={e => e.stopPropagation()}>
          <div className="st-header">
            <IconSettings2 stroke={2} size={20} />
            <span className="st-title">Настройки</span>
            <button type="button" className="st-close" onClick={onClose} aria-label="Закрыть"><IconX size={18} /></button>
          </div>

          <div className="st-row">
            <span>Запускать при старте</span>
            <button type="button" className={`st-toggle st-toggle--${settings.autoStart ? 'on' : 'off'}`} aria-label={settings.autoStart ? 'Отключить автозапуск' : 'Включить автозапуск'} onClick={() => {
              const next = !settings.autoStart;
              update('autoStart', next);
              SetAutoStart(next);
            }} />
          </div>

          <div className="st-row">
            <span>Режим обфускации {!settings.obfsAccepted && <span className="st-badge">только latest</span>}</span>
            <div className="st-segment">
              <button type="button" className={`st-seg-btn${settings.obfsMode === 'audio' ? ' st-seg-btn--active' : ''}`} onClick={() => handleObfsClick('audio')}>Audio</button>
              <button type="button" className={`st-seg-btn${settings.obfsMode === 'video' ? ' st-seg-btn--active' : ''}`} onClick={() => handleObfsClick('video')}>Video</button>
            </div>
          </div>

          <div className="st-row">
            <span>Транспорт TURN</span>
            <div className="st-segment">
              <button type="button" className={`st-seg-btn${!settings.turnTcp ? ' st-seg-btn--active' : ''}`} onClick={() => update('turnTcp', false)}>UDP</button>
              <button type="button" className={`st-seg-btn${settings.turnTcp ? ' st-seg-btn--active' : ''}`} onClick={() => update('turnTcp', true)}>TCP</button>
            </div>
          </div>

          <div className="st-info">
            <div className="st-info-name">PWDTT</div>
            <div className="st-info-ver">v{version}</div>
          </div>

          <div className="st-actions">
            <button type="button" className={`st-action${copiedReport ? ' st-action--copied' : ''}`} onClick={handleReport}>
              <IconBug size={14} />
              {copiedReport ? 'Скопировано!' : 'Отчёт'}
            </button>
          </div>

          <button type="button" className="st-donate" onClick={() => BrowserOpenURL('https://pay.cloudtips.ru/p/1ea077ba')}>
            <IconHeartFilled size={16} />
            Поддержать проект
          </button>
        </div>
      </div>

      {showObfsModal && (
        <div className="st-overlay" onClick={() => setShowObfsModal(false)}>
          <div className="st-modal st-obfs-modal" onClick={e => e.stopPropagation()}>
            <div className="st-obfs-icon">
              <IconAlertTriangle size={32} />
            </div>
            <div className="st-obfs-title">Режим обфускации</div>
            <div className="st-obfs-text">
              Обфускация поддерживается только на последней версии сервера WDTT.
              При использовании на старых серверах соединение может не состояться.
              Если вы не уверены в совместимости, рекомендуется использовать режим "Audio".
            </div>
            <div className="st-obfs-text st-obfs-disclaimer">
              Разработчик не несёт ответственности за проблемы при использовании на несовместимых серверах.
            </div>
            <div className="st-obfs-actions">
              <button type="button" className="st-obfs-btn st-obfs-btn--cancel" onClick={() => setShowObfsModal(false)}>Отмена</button>
              <button type="button" className="st-obfs-btn st-obfs-btn--accept" onClick={handleObfsAccept}>Принимаю</button>
            </div>
          </div>
        </div>
      )}
    </>
  );
}
