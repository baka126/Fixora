import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { ShieldAlert, Loader2, KeyRound } from 'lucide-react';
import { useStore } from '../store/useStore';
import { apiClient } from '../api/client';

export const Setup: React.FC = () => {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const navigate = useNavigate();
  const { setToken, setUser } = useStore();

  const handleSetup = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setLoading(true);

    try {
      const { data } = await apiClient.post('/auth/setup', { username, password });
      setToken(data.token);
      setUser(data.user);
      navigate('/');
    } catch (err: unknown) {
      const message = typeof err === 'object' && err !== null && 'response' in err
        ? (err as { response?: { data?: string } }).response?.data
        : undefined;
      setError(message || 'Setup failed. Please try again.');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center bg-[#f3f6f8] px-4">
      <div className="max-w-md w-full bg-white rounded-xl shadow-lg border border-[#d8e1e8] p-8">
        <div className="flex flex-col items-center mb-8">
          <div className="w-12 h-12 bg-[#d8f2ed] text-[#0e665f] rounded-lg flex items-center justify-center font-bold text-xl mb-4">
            <KeyRound className="w-6 h-6" />
          </div>
          <h1 className="text-2xl font-bold text-[#162033]">Welcome to Fixora</h1>
          <p className="text-[#647084] mt-2 text-center">Let's create your initial admin account.</p>
        </div>

        {error && (
          <div className="mb-6 p-4 bg-red-50 border border-red-200 rounded-lg flex items-start gap-3 text-red-700">
            <ShieldAlert className="w-5 h-5 flex-shrink-0 mt-0.5" />
            <p className="text-sm">{error}</p>
          </div>
        )}

        <form onSubmit={handleSetup} className="space-y-5">
          <div>
            <label className="block text-sm font-medium text-[#162033] mb-1">Admin Username</label>
            <input
              type="text"
              required
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              className="w-full px-4 py-2 border border-[#d8e1e8] rounded-lg focus:ring-2 focus:ring-[#157f7d] focus:border-[#157f7d] outline-none transition-colors"
              placeholder="admin"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-[#162033] mb-1">Admin Password</label>
            <input
              type="password"
              required
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className="w-full px-4 py-2 border border-[#d8e1e8] rounded-lg focus:ring-2 focus:ring-[#157f7d] focus:border-[#157f7d] outline-none transition-colors"
              placeholder="••••••••"
            />
          </div>
          <button
            type="submit"
            disabled={loading}
            className="w-full bg-[#157f7d] hover:bg-[#0f6664] text-white font-medium py-2.5 rounded-lg transition-colors flex items-center justify-center gap-2 disabled:opacity-70"
          >
            {loading ? <Loader2 className="w-5 h-5 animate-spin" /> : 'Complete Setup'}
          </button>
        </form>
      </div>
    </div>
  );
};
