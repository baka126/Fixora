import { useEffect, useRef } from 'react';
import { apiClient, getApiBaseURL } from '../api/client';
import { useStore } from '../store/useStore';
import type { DashboardState } from '../types';

const DASHBOARD_REFRESH_INTERVAL_MS = 10_000;
const WS_RECONNECT_INITIAL_MS = 1_000;
const WS_RECONNECT_MAX_MS = 30_000;

export const useWebSocket = () => {
  const ws = useRef<WebSocket | null>(null);
  const reconnectTimer = useRef<number | null>(null);
  const reconnectAttempt = useRef(0);
  const { token, setDashboard } = useStore();

  useEffect(() => {
    if (!token) return;

    let closedByEffect = false;
    const refreshDashboard = () => {
      apiClient
        .get('/dashboard')
        .then(({ data }: { data: DashboardState }) => {
          if (!closedByEffect) {
            setDashboard(data);
          }
        })
        .catch(() => undefined);
    };

    const scheduleReconnect = () => {
      if (closedByEffect || reconnectTimer.current !== null) return;
      const delay = Math.min(WS_RECONNECT_INITIAL_MS * 2 ** reconnectAttempt.current, WS_RECONNECT_MAX_MS);
      reconnectAttempt.current += 1;
      reconnectTimer.current = window.setTimeout(() => {
        reconnectTimer.current = null;
        connect();
      }, delay);
    };

    const connect = () => {
      const socket = new WebSocket(webSocketURL(token));
      ws.current = socket;

      socket.onopen = () => {
        reconnectAttempt.current = 0;
        refreshDashboard();
      };

      socket.onmessage = (event) => {
        try {
          const payload = JSON.parse(event.data);
          if (payload && typeof payload === 'object') {
            setDashboard(payload);
          }
        } catch (err) {
          console.error('Failed to parse websocket message', err);
        }
      };

      socket.onerror = () => {
        refreshDashboard();
      };

      socket.onclose = () => {
        refreshDashboard();
        scheduleReconnect();
      };
    };

    refreshDashboard();
    connect();

    return () => {
      closedByEffect = true;
      if (reconnectTimer.current !== null) {
        window.clearTimeout(reconnectTimer.current);
        reconnectTimer.current = null;
      }
      if (ws.current?.readyState === WebSocket.OPEN || ws.current?.readyState === WebSocket.CONNECTING) {
        ws.current.close();
      }
    };
  }, [token, setDashboard]);

  useEffect(() => {
    if (!token) return;

    let cancelled = false;
    const intervalId = window.setInterval(() => {
      apiClient
        .get('/dashboard')
        .then(({ data }) => {
          if (!cancelled) {
            setDashboard(data);
          }
        })
        .catch(() => {
          // Polling keeps dashboard data fresh when a proxy drops WebSocket upgrades.
        });
    }, DASHBOARD_REFRESH_INTERVAL_MS);

    return () => {
      cancelled = true;
      window.clearInterval(intervalId);
    };
  }, [token, setDashboard]);

  return null;
};

const webSocketURL = (token: string) => {
  const apiBase = getApiBaseURL();
  const url = new URL(`${apiBase.replace(/\/$/, '')}/ws`, window.location.origin);
  url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:';
  url.searchParams.set('token', token);
  return url.toString();
};
