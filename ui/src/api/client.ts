import axios from 'axios';
import { useStore } from '../store/useStore';

declare global {
  interface Window {
    FIXORA_CONFIG?: {
      apiBaseUrl: string;
    };
  }
}

const getBaseURL = () => {
  const configBase = window.FIXORA_CONFIG?.apiBaseUrl || '/api';
  // Ensure it ends with /v1
  return configBase.endsWith('/v1') ? configBase : `${configBase.replace(/\/$/, '')}/v1`;
};

export const apiClient = axios.create({
  baseURL: getBaseURL(),
  headers: {
    'Content-Type': 'application/json',
  },
});

apiClient.interceptors.request.use((config) => {
  const token = useStore.getState().token;
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

apiClient.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      useStore.getState().logout();
      window.location.href = '/login';
    }
    return Promise.reject(error);
  }
);
