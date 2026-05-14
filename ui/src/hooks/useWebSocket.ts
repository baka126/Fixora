import { useEffect, useRef } from 'react';
import { useStore } from '../store/useStore';

export const useWebSocket = () => {
  const ws = useRef<WebSocket | null>(null);
  const { token, setDashboard } = useStore();

  useEffect(() => {
    if (!token) return;

    // Use ws:// for local dev, wss:// for production if TLS enabled
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    // When using Vite proxy, the WS connection must go to the Vite port and let it proxy,
    // but typically Vite doesn't proxy WS out-of-the-box unless configured.
    // However, our proxy config has `changeOrigin: true`. Let's connect directly to the backend
    // for simplicity or rely on relative path if we map it. 
    // Vite proxy handles WS: "ws://localhost:5173/api/v1/ws" will be proxied to "ws://localhost:8080/api/v1/ws"
    const wsUrl = `${protocol}//${window.location.host}/api/v1/ws?token=${token}`;
    
    ws.current = new WebSocket(wsUrl);

    ws.current.onopen = () => {
      console.log('WebSocket connected');
    };

    ws.current.onmessage = (event) => {
      try {
        const payload = JSON.parse(event.data);
        if (payload && typeof payload === 'object') {
          // Assuming payload is the new dashboard state
          setDashboard(payload);
        }
      } catch (err) {
        console.error('Failed to parse websocket message', err);
      }
    };

    ws.current.onerror = (error) => {
      console.error('WebSocket error:', error);
    };

    ws.current.onclose = () => {
      console.log('WebSocket disconnected');
    };

    return () => {
      if (ws.current?.readyState === WebSocket.OPEN) {
        ws.current.close();
      }
    };
  }, [token, setDashboard]);
  return null;
};
