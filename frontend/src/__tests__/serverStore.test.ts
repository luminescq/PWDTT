import { describe, it, expect, beforeEach } from 'vitest';
import { serverStore } from '../lib/store';

beforeEach(() => {
  localStorage.clear();
});

describe('serverStore', () => {
  it('getAll: пустой localStorage → []', () => {
    expect(serverStore.getAll()).toEqual([]);
  });

  it('add: генерация UUID', () => {
    const server = serverStore.add({
      name: 'Test',
      host: '1.2.3.4:5555',
      password: 'secret',
    });

    expect(server.id).toBeTruthy();
    expect(server.id.length).toBeGreaterThan(0);
    expect(server.name).toBe('Test');
    expect(server.host).toBe('1.2.3.4:5555');
  });

  it('add: сохраняется в localStorage', () => {
    serverStore.add({
      name: 'A',
      host: '1.1.1.1:1111',
      password: 'pw',
    });

    const all = serverStore.getAll();
    expect(all.length).toBe(1);
    expect(all[0].name).toBe('A');
  });

  it('add: сохраняет мощность из профиля', () => {
    serverStore.add({ name: 'A', host: '1.1.1.1:1111', password: 'pw', power: 18 });

    expect(serverStore.getAll()[0].power).toBe(18);
  });

  it('add: несколько серверов', () => {
    serverStore.add({ name: 'A', host: 'a:1', password: 'p1' });
    serverStore.add({ name: 'B', host: 'b:2', password: 'p2' });

    expect(serverStore.getAll().length).toBe(2);
  });

  it('update: обновление существующего сервера', () => {
    const server = serverStore.add({
      name: 'Old',
      host: '1.1.1.1:1111',
      password: 'pw',
    });

    serverStore.update({ ...server, name: 'New', host: '2.2.2.2:2222' });

    const all = serverStore.getAll();
    expect(all.length).toBe(1);
    expect(all[0].name).toBe('New');
    expect(all[0].host).toBe('2.2.2.2:2222');
  });

  it('remove: удаление по id', () => {
    const s1 = serverStore.add({ name: 'A', host: 'a:1', password: 'p1' });
    const s2 = serverStore.add({ name: 'B', host: 'b:2', password: 'p2' });

    serverStore.remove(s1.id);

    const all = serverStore.getAll();
    expect(all.length).toBe(1);
    expect(all[0].id).toBe(s2.id);
  });

  it('remove: несуществующий id → ничего не меняется', () => {
    serverStore.add({ name: 'A', host: 'a:1', password: 'p1' });
    serverStore.remove('nonexistent-id');

    expect(serverStore.getAll().length).toBe(1);
  });

  it('getLastSelectedId: пустой → null', () => {
    expect(serverStore.getLastSelectedId()).toBeNull();
  });

  it('setLastSelectedId / getLastSelectedId', () => {
    serverStore.setLastSelectedId('abc-123');
    expect(serverStore.getLastSelectedId()).toBe('abc-123');
  });

  it('setLastSelectedId(null) → удаление', () => {
    serverStore.setLastSelectedId('abc-123');
    serverStore.setLastSelectedId(null);
    expect(serverStore.getLastSelectedId()).toBeNull();
  });

  it('parse с невалидным JSON → fallback', () => {
    localStorage.setItem('wdtt_servers:v1', '{invalid json');
    expect(serverStore.getAll()).toEqual([]);

    localStorage.setItem('wdtt_settings:v1', 'not-json');
    // settingsStore.get() tested in settingsStore.test.ts
  });
});
