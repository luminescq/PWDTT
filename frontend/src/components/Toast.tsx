import { useEffect, useState } from 'react';
import { toastStore } from '../lib/stores/toastStore';
import './Toast.css';

export default function Toast() {
  const [msg, setMsg] = useState<string | null>(null);
  useEffect(() => toastStore.subscribe(setMsg), []);
  if (!msg) return null;
  return <div className="toast">{msg}</div>;
}
