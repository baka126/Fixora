import React, { useEffect, useState, useCallback } from 'react';
import { X, Shield, UserPlus, FolderPlus, Trash2 } from 'lucide-react';
import { apiClient } from '../api/client';
import type { User, Group } from '../types';
import { useStore } from '../store/useStore';

interface Props {
  onClose: () => void;
}

export const UserManagementPanel: React.FC<Props> = ({ onClose }) => {
  const [activeTab, setActiveTab] = useState<'users' | 'groups'>('users');
  const [users, setUsers] = useState<User[]>([]);
  const [groups, setGroups] = useState<Group[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Forms
  const [newUsername, setNewUsername] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [newRole, setNewRole] = useState<'admin' | 'operator' | 'viewer'>('viewer');
  
  const [newGroupName, setNewGroupName] = useState('');
  const [newGroupDesc, setNewGroupDesc] = useState('');

  const currentUser = useStore(state => state.user);

  const fetchData = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [usersRes, groupsRes] = await Promise.all([
        apiClient.get<User[]>('/auth/users'),
        apiClient.get<Group[]>('/auth/groups')
      ]);
      setUsers(usersRes.data || []);
      setGroups(groupsRes.data || []);
    } catch (err) {
      const errorMsg = err instanceof Error && 'response' in err 
        ? (err as { response?: { data?: string } }).response?.data 
        : 'Failed to fetch users and groups. Ensure you are an Admin.';
      setError(errorMsg || 'Failed to fetch users and groups.');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    fetchData();
  }, [fetchData]);

  const handleCreateUser = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      await apiClient.post('/auth/users', { username: newUsername, password: newPassword, role: newRole });
      setNewUsername('');
      setNewPassword('');
       
    fetchData();
    } catch (err) {
      const errorMsg = err instanceof Error && 'response' in err ? (err as { response?: { data?: string } }).response?.data : 'Failed to create user';
      setError(errorMsg || 'Failed to create user');
    }
  };

  const handleDeleteUser = async (id: string) => {
    if (!window.confirm('Are you sure you want to delete this user?')) return;
    try {
      await apiClient.delete(`/auth/users/${id}`);
       
    fetchData();
    } catch (err) {
      const errorMsg = err instanceof Error && 'response' in err ? (err as { response?: { data?: string } }).response?.data : 'Failed to delete user';
      setError(errorMsg || 'Failed to delete user');
    }
  };

  const handleUpdateRole = async (id: string, role: string) => {
    try {
      await apiClient.put(`/auth/users/${id}/role`, { role });
       
    fetchData();
    } catch (err) {
      const errorMsg = err instanceof Error && 'response' in err ? (err as { response?: { data?: string } }).response?.data : 'Failed to update role';
      setError(errorMsg || 'Failed to update role');
    }
  };

  const handleCreateGroup = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      await apiClient.post('/auth/groups', { name: newGroupName, description: newGroupDesc });
      setNewGroupName('');
      setNewGroupDesc('');
       
    fetchData();
    } catch (err) {
      const errorMsg = err instanceof Error && 'response' in err ? (err as { response?: { data?: string } }).response?.data : 'Failed to create group';
      setError(errorMsg || 'Failed to create group');
    }
  };

  const handleDeleteGroup = async (id: string) => {
    if (!window.confirm('Are you sure you want to delete this group?')) return;
    try {
      await apiClient.delete(`/auth/groups/${id}`);
       
    fetchData();
    } catch (err) {
      const errorMsg = err instanceof Error && 'response' in err ? (err as { response?: { data?: string } }).response?.data : 'Failed to delete group';
      setError(errorMsg || 'Failed to delete group');
    }
  };

  const handleToggleUserGroup = async (userId: string, groupId: string, isMember: boolean) => {
    try {
      if (isMember) {
        await apiClient.delete(`/auth/groups/${groupId}/users/${userId}`);
      } else {
        await apiClient.post(`/auth/groups/${groupId}/users/${userId}`);
      }
       
    fetchData();
    } catch (err) {
      const errorMsg = err instanceof Error && 'response' in err ? (err as { response?: { data?: string } }).response?.data : 'Failed to toggle group membership';
      setError(errorMsg || 'Failed to toggle group membership');
    }
  };

  return (
    <div className="fixed inset-y-0 right-0 z-[100] flex max-w-full pl-10 bg-black/30 backdrop-blur-sm">
      <div className="w-screen max-w-3xl transform bg-[#f8fafc] shadow-2xl transition-transform duration-500 ease-in-out sm:duration-700">
        <div className="flex h-full flex-col overflow-y-scroll shadow-xl">
          <header className="bg-[#1e293b] px-4 py-6 sm:px-6 shadow-md z-10">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-3">
                <div className="rounded-lg bg-[#334155] p-2 text-white">
                  <Shield className="h-5 w-5" />
                </div>
                <div>
                  <h2 className="text-lg font-semibold text-white">User & Access Management</h2>
                  <p className="text-sm text-[#94a3b8]">Manage roles, groups, and access permissions</p>
                </div>
              </div>
              <button onClick={onClose} className="rounded-md text-[#94a3b8] hover:text-white outline-none">
                <X className="h-6 w-6" />
              </button>
            </div>
            
            <div className="flex gap-6 mt-6 border-b border-[#334155]">
              <button
                onClick={() => setActiveTab('users')}
                className={`pb-3 text-sm font-medium transition-colors ${activeTab === 'users' ? 'text-white border-b-2 border-[#157f7d]' : 'text-[#94a3b8] hover:text-white'}`}
              >
                Users & Roles
              </button>
              <button
                onClick={() => setActiveTab('groups')}
                className={`pb-3 text-sm font-medium transition-colors ${activeTab === 'groups' ? 'text-white border-b-2 border-[#157f7d]' : 'text-[#94a3b8] hover:text-white'}`}
              >
                Groups
              </button>
            </div>
          </header>

          <div className="flex-1 p-6">
            {error && (
              <div className="mb-6 rounded-lg bg-red-50 p-4 text-sm text-red-700 border border-red-200 flex items-center justify-between">
                <span>{error}</span>
                <button onClick={() => setError(null)}><X className="h-4 w-4" /></button>
              </div>
            )}

            {loading ? (
              <div className="flex h-32 items-center justify-center">
                <Loader2 className="h-8 w-8 animate-spin text-[#157f7d]" />
              </div>
            ) : (
              <>
                {activeTab === 'users' && (
                  <div className="space-y-8">
                    {/* Create User Form */}
                    <div className="bg-white p-5 rounded-xl border border-[#e2e8f0] shadow-sm">
                      <div className="flex items-center gap-2 mb-4 text-[#0f172a]">
                        <UserPlus className="h-4 w-4" />
                        <h3 className="font-semibold text-sm">Add New User</h3>
                      </div>
                      <form onSubmit={handleCreateUser} className="grid grid-cols-1 md:grid-cols-4 gap-4 items-end">
                        <div>
                          <label className="block text-[11px] font-bold text-[#647084] uppercase mb-1">Username</label>
                          <input required type="text" value={newUsername} onChange={e => setNewUsername(e.target.value)} className="w-full rounded-md border border-[#cbd5e1] px-3 py-1.5 text-sm focus:border-[#157f7d] outline-none" />
                        </div>
                        <div>
                          <label className="block text-[11px] font-bold text-[#647084] uppercase mb-1">Password</label>
                          <input required type="password" value={newPassword} onChange={e => setNewPassword(e.target.value)} className="w-full rounded-md border border-[#cbd5e1] px-3 py-1.5 text-sm focus:border-[#157f7d] outline-none" />
                        </div>
                        <div>
                          <label className="block text-[11px] font-bold text-[#647084] uppercase mb-1">Role</label>
                          <select value={newRole} onChange={e => setNewRole(e.target.value as 'admin'|'operator'|'viewer')} className="w-full rounded-md border border-[#cbd5e1] px-3 py-1.5 text-sm focus:border-[#157f7d] outline-none">
                            <option value="viewer">Viewer</option>
                            <option value="operator">Operator</option>
                            <option value="admin">Admin</option>
                          </select>
                        </div>
                        <button type="submit" className="bg-[#157f7d] text-white rounded-md px-4 py-1.5 text-sm font-medium hover:bg-[#0f6664] transition-colors">Create User</button>
                      </form>
                    </div>

                    {/* Users Table */}
                    <div className="bg-white rounded-xl border border-[#e2e8f0] shadow-sm overflow-hidden">
                      <table className="w-full text-left text-[13px]">
                        <thead className="bg-[#f8fafc] text-[12px] text-[#647084] uppercase">
                          <tr>
                            <th className="px-5 py-3 font-semibold">User</th>
                            <th className="px-5 py-3 font-semibold">Role</th>
                            <th className="px-5 py-3 font-semibold">Groups</th>
                            <th className="px-5 py-3 font-semibold text-right">Actions</th>
                          </tr>
                        </thead>
                        <tbody className="divide-y divide-[#e2e8f0]">
                          {users.map(u => (
                            <tr key={u.id} className="hover:bg-[#f8fafc]">
                              <td className="px-5 py-3 font-medium text-[#0f172a]">{u.username}</td>
                              <td className="px-5 py-3">
                                <select 
                                  value={u.role} 
                                  onChange={(e) => handleUpdateRole(u.id, e.target.value)}
                                  disabled={u.id === currentUser?.id}
                                  className="rounded border border-[#e2e8f0] bg-white px-2 py-1 text-xs outline-none focus:border-[#157f7d]"
                                >
                                  <option value="admin">Admin</option>
                                  <option value="operator">Operator</option>
                                  <option value="viewer">Viewer</option>
                                </select>
                              </td>
                              <td className="px-5 py-3">
                                <div className="flex flex-wrap gap-1">
                                  {u.groups?.map(g => (
                                    <span key={g.id} className="inline-flex items-center rounded-full bg-[#eff6ff] px-2 py-0.5 text-[10px] font-medium text-[#1d4ed8]">
                                      {g.name}
                                    </span>
                                  ))}
                                  {(!u.groups || u.groups.length === 0) && <span className="text-[#94a3b8] text-xs">No groups</span>}
                                </div>
                              </td>
                              <td className="px-5 py-3 text-right">
                                {u.id !== currentUser?.id && (
                                  <button onClick={() => handleDeleteUser(u.id)} className="text-red-500 hover:text-red-700 p-1 rounded-md hover:bg-red-50">
                                    <Trash2 className="h-4 w-4" />
                                  </button>
                                )}
                              </td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                  </div>
                )}

                {activeTab === 'groups' && (
                  <div className="space-y-8">
                    {/* Create Group Form */}
                    <div className="bg-white p-5 rounded-xl border border-[#e2e8f0] shadow-sm">
                      <div className="flex items-center gap-2 mb-4 text-[#0f172a]">
                        <FolderPlus className="h-4 w-4" />
                        <h3 className="font-semibold text-sm">Create New Group</h3>
                      </div>
                      <form onSubmit={handleCreateGroup} className="grid grid-cols-1 md:grid-cols-5 gap-4 items-end">
                        <div className="md:col-span-2">
                          <label className="block text-[11px] font-bold text-[#647084] uppercase mb-1">Group Name</label>
                          <input required type="text" value={newGroupName} onChange={e => setNewGroupName(e.target.value)} className="w-full rounded-md border border-[#cbd5e1] px-3 py-1.5 text-sm focus:border-[#157f7d] outline-none" />
                        </div>
                        <div className="md:col-span-2">
                          <label className="block text-[11px] font-bold text-[#647084] uppercase mb-1">Description</label>
                          <input type="text" value={newGroupDesc} onChange={e => setNewGroupDesc(e.target.value)} className="w-full rounded-md border border-[#cbd5e1] px-3 py-1.5 text-sm focus:border-[#157f7d] outline-none" />
                        </div>
                        <button type="submit" className="bg-[#157f7d] text-white rounded-md px-4 py-1.5 text-sm font-medium hover:bg-[#0f6664] transition-colors">Create</button>
                      </form>
                    </div>

                    {/* Groups Table */}
                    <div className="bg-white rounded-xl border border-[#e2e8f0] shadow-sm overflow-hidden">
                      <table className="w-full text-left text-[13px]">
                        <thead className="bg-[#f8fafc] text-[12px] text-[#647084] uppercase">
                          <tr>
                            <th className="px-5 py-3 font-semibold w-1/3">Group</th>
                            <th className="px-5 py-3 font-semibold">Members</th>
                            <th className="px-5 py-3 font-semibold text-right">Actions</th>
                          </tr>
                        </thead>
                        <tbody className="divide-y divide-[#e2e8f0]">
                          {groups.map(g => (
                            <tr key={g.id} className="hover:bg-[#f8fafc]">
                              <td className="px-5 py-3">
                                <div className="font-medium text-[#0f172a]">{g.name}</div>
                                <div className="text-[11px] text-[#647084] mt-0.5">{g.description}</div>
                              </td>
                              <td className="px-5 py-3">
                                <div className="flex flex-col gap-2">
                                  <div className="flex flex-wrap gap-1">
                                    {users.filter(u => u.groups?.some(ug => ug.id === g.id)).map(u => (
                                      <span key={u.id} className="inline-flex items-center gap-1 rounded-full bg-[#f1f5f9] px-2 py-0.5 text-[11px] text-[#334155] border border-[#e2e8f0]">
                                        {u.username}
                                        <button onClick={() => handleToggleUserGroup(u.id, g.id, true)} className="hover:text-red-500"><X className="h-3 w-3" /></button>
                                      </span>
                                    ))}
                                  </div>
                                  <select 
                                    className="rounded border border-[#e2e8f0] bg-white px-2 py-1 text-xs outline-none w-max text-[#647084]"
                                    onChange={(e) => {
                                      if (e.target.value) {
                                        handleToggleUserGroup(e.target.value, g.id, false);
                                        e.target.value = "";
                                      }
                                    }}
                                  >
                                    <option value="">+ Add member...</option>
                                    {users.filter(u => !u.groups?.some(ug => ug.id === g.id)).map(u => (
                                      <option key={u.id} value={u.id}>{u.username}</option>
                                    ))}
                                  </select>
                                </div>
                              </td>
                              <td className="px-5 py-3 text-right align-top">
                                <button onClick={() => handleDeleteGroup(g.id)} className="text-red-500 hover:text-red-700 p-1 rounded-md hover:bg-red-50 mt-1">
                                  <Trash2 className="h-4 w-4" />
                                </button>
                              </td>
                            </tr>
                          ))}
                          {groups.length === 0 && (
                            <tr>
                              <td colSpan={3} className="px-5 py-8 text-center text-[#647084]">No groups created yet.</td>
                            </tr>
                          )}
                        </tbody>
                      </table>
                    </div>
                  </div>
                )}
              </>
            )}
          </div>
        </div>
      </div>
    </div>
  );
};

// Also adding a Loader component to avoid importing missing icons if it happens
const Loader2 = ({ className }: { className?: string }) => (
  <svg className={`animate-spin ${className}`} xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
    <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
    <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
  </svg>
);
