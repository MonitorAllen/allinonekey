<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue';
import { useRouter } from 'vue-router';
import {
  Activity,
  AlertCircle,
  Check,
  CheckCircle2,
  Copy,
  Edit3,
  ExternalLink,
  FileDown,
  Globe,
  Key,
  Lock,
  LogOut,
  Plus,
  RefreshCw,
  Search,
  ServerCog,
  ShieldCheck,
  Ticket,
  Trash2,
  Upload,
  X,
} from 'lucide-vue-next';
import api, { getApiErrorMessage } from '../api';
import { useAuthStore } from '../store/auth';

type ToastType = 'success' | 'error';
type SecretType = 'keys' | 'accounts';
type ExportScope = 'all' | 'keys' | 'accounts';
type ExportFormat = 'json' | 'csv';

type KeyRow = { key_name: string; key_value: string };
type KeyForm = {
  provider: string;
  custom_provider: string;
  pool_group: string;
  base_url: string;
  proxy_url: string;
  status: string;
  keys: KeyRow[];
};
type AccountForm = {
  platform: string;
  account: string;
  password: string;
  url: string;
  totp_secret: string;
  favicon_url: string;
};

const router = useRouter();
const auth = useAuthStore();
const activeTab = ref('keys');
const keys = ref<any[]>([]);
const accounts = ref<any[]>([]);
const invites = ref<any[]>([]);
const auditLogs = ref<any[]>([]);
const keyStats = ref({ total: 0, active: 0, error: 0, balance: 0 });
const searchQuery = ref('');
const copiedId = ref<string | null>(null);
const toast = ref<{ message: string; type: ToastType } | null>(null);
const CLIPBOARD_CLEAR_DELAY_MS = 30000;
let clipboardClearTimer: ReturnType<typeof setTimeout> | null = null;

const loadingStats = ref(false);
const loadingKeys = ref(false);
const loadingAccounts = ref(false);
const loadingLogs = ref(false);
const loadingInvites = ref(false);

const showKeyModal = ref(false);
const editingKeyId = ref<number | null>(null);
const showAccountModal = ref(false);
const editingAccountId = ref<number | null>(null);
const showModelsModal = ref(false);
const modelListTitle = ref('');
const loadingModels = ref(false);
const modelListStatus = ref('');
const modelListError = ref('');
const keyModels = ref<any[]>([]);
const importFile = ref<File | null>(null);

const auditFilters = reactive({ action: '', keyword: '' });
const auditPage = ref(1);
const auditPageSize = ref(20);
const auditTotal = ref(0);
const auditTotalPages = computed(() => Math.max(1, Math.ceil(auditTotal.value / auditPageSize.value)));
const inviteFilters = reactive({ status: '' });
const invitePage = ref(1);
const invitePageSize = ref(20);
const inviteTotal = ref(0);
const inviteTotalPages = computed(() => Math.max(1, Math.ceil(inviteTotal.value / invitePageSize.value)));

const keyForm = reactive<KeyForm>({
  provider: 'OpenAI',
  custom_provider: '',
  pool_group: 'default',
  base_url: '',
  proxy_url: '',
  status: 'active',
  keys: [{ key_name: '', key_value: '' }],
});
const accountForm = reactive<AccountForm>({ platform: '', account: '', password: '', url: '', totp_secret: '', favicon_url: '' });
const inviteExpiresInHours = ref(168);
const totpCodes = reactive<Record<number, { code: string; remaining: number }>>({});

const isKeysTab = computed(() => activeTab.value === 'keys');
const isAccountsTab = computed(() => activeTab.value === 'accounts');
const activeExportScope = computed<ExportScope>(() => (isKeysTab.value ? 'keys' : isAccountsTab.value ? 'accounts' : 'all'));
const keyModalTitle = computed(() => (editingKeyId.value ? 'Edit API Key' : 'Add API Keys'));
const accountModalTitle = computed(() => (editingAccountId.value ? 'Edit Account' : 'New Account'));

const showToast = (message: string, type: ToastType = 'success') => {
  toast.value = { message, type };
  setTimeout(() => {
    if (toast.value?.message === message) toast.value = null;
  }, 3000);
};

const handleError = (error: unknown) => {
  const message = getApiErrorMessage(error);
  showToast(message, 'error');
  if (message === 'Unauthorized' || message === 'Missing token') {
    auth.logout();
    router.push('/login');
  }
};

const resetKeyForm = () => {
  editingKeyId.value = null;
  keyForm.provider = 'OpenAI';
  keyForm.custom_provider = '';
  keyForm.pool_group = 'default';
  keyForm.base_url = '';
  keyForm.proxy_url = '';
  keyForm.status = 'active';
  keyForm.keys = [{ key_name: '', key_value: '' }];
};

const resetAccountForm = () => {
  editingAccountId.value = null;
  accountForm.platform = '';
  accountForm.account = '';
  accountForm.password = '';
  accountForm.url = '';
  accountForm.totp_secret = '';
  accountForm.favicon_url = '';
};

const providerName = () => (keyForm.provider === 'Custom' ? keyForm.custom_provider.trim() : keyForm.provider);
const formatDate = (value: string) => (value && !value.startsWith('0001-') ? new Date(value).toLocaleString() : 'Never');
const keyStatusClass = (status: string) => {
  if (status === 'active') return 'text-green-400 bg-green-400/10 border-green-400/20';
  if (status === 'quota_unsupported') return 'text-gray-400 bg-gray-400/10 border-gray-400/20';
  if (status === 'rate_limited') return 'text-yellow-400 bg-yellow-400/10 border-yellow-400/20';
  return 'text-red-400 bg-red-400/10 border-red-400/20';
};

const scheduleClipboardClear = (expectedValue: string) => {
  if (clipboardClearTimer) clearTimeout(clipboardClearTimer);
  clipboardClearTimer = setTimeout(async () => {
    try {
      const currentValue = await navigator.clipboard.readText();
      if (currentValue === expectedValue) {
        await navigator.clipboard.writeText('');
        showToast('Clipboard cleared');
      }
    } catch {
      // Clipboard reads can be blocked by browser focus/permission rules.
    } finally {
      clipboardClearTimer = null;
    }
  }, CLIPBOARD_CLEAR_DELAY_MS);
};

const fetchStats = async () => {
  loadingStats.value = true;
  try {
    const res = await api.get('/keys/stats');
    keyStats.value = res.data || { total: 0, active: 0, error: 0, balance: 0 };
  } catch (error) {
    handleError(error);
  } finally {
    loadingStats.value = false;
  }
};

const fetchKeys = async () => {
  loadingKeys.value = true;
  try {
    const res = await api.get('/keys/list', { params: { q: searchQuery.value } });
    keys.value = res.data || [];
  } catch (error) {
    handleError(error);
  } finally {
    loadingKeys.value = false;
  }
};

const fetchAccounts = async () => {
  loadingAccounts.value = true;
  try {
    const res = await api.get('/accounts/list');
    accounts.value = res.data || [];
  } catch (error) {
    handleError(error);
  } finally {
    loadingAccounts.value = false;
  }
};

const fetchLogs = async () => {
  loadingLogs.value = true;
  try {
    const res = await api.get('/audit/list', {
      params: { page: auditPage.value, page_size: auditPageSize.value, action: auditFilters.action || undefined, keyword: auditFilters.keyword || undefined },
    });
    auditLogs.value = res.data?.items || [];
    auditTotal.value = res.data?.total || 0;
    auditPage.value = res.data?.page || auditPage.value;
    auditPageSize.value = res.data?.page_size || auditPageSize.value;
  } catch (error) {
    handleError(error);
  } finally {
    loadingLogs.value = false;
  }
};

const fetchInvites = async () => {
  if (auth.role !== 'admin') return;
  loadingInvites.value = true;
  try {
    const res = await api.get('/admin/invites', { params: { page: invitePage.value, page_size: invitePageSize.value, status: inviteFilters.status || undefined } });
    invites.value = res.data?.items || [];
    inviteTotal.value = res.data?.total || 0;
    invitePage.value = res.data?.page || invitePage.value;
    invitePageSize.value = res.data?.page_size || invitePageSize.value;
  } catch (error) {
    handleError(error);
  } finally {
    loadingInvites.value = false;
  }
};

const refreshActiveTab = () => {
  if (activeTab.value === 'keys') {
    fetchKeys();
    fetchStats();
  } else if (activeTab.value === 'accounts') fetchAccounts();
  else if (activeTab.value === 'logs') fetchLogs();
  else if (activeTab.value === 'admin') fetchInvites();
};

onMounted(refreshActiveTab);
watch(activeTab, refreshActiveTab);
watch(searchQuery, () => {
  if (activeTab.value === 'keys') fetchKeys();
});
watch([() => auditFilters.action, () => auditFilters.keyword], () => {
  if (activeTab.value !== 'logs') return;
  auditPage.value = 1;
  fetchLogs();
});
watch(() => inviteFilters.status, () => {
  if (activeTab.value !== 'admin') return;
  invitePage.value = 1;
  fetchInvites();
});

const groupedKeys = computed(() => {
  const groups: Record<string, Record<string, any[]>> = {};
  keys.value.forEach(k => {
    if (!groups[k.provider]) groups[k.provider] = {};
    const group = k.pool_group || 'default';
    if (!groups[k.provider][group]) groups[k.provider][group] = [];
    groups[k.provider][group].push(k);
  });
  return groups;
});

const openNewKeyModal = () => {
  resetKeyForm();
  showKeyModal.value = true;
};

const openEditKeyModal = (key: any) => {
  editingKeyId.value = key.id;
  keyForm.provider = ['OpenAI', 'DeepSeek', 'Anthropic', 'Gemini'].includes(key.provider) ? key.provider : 'Custom';
  keyForm.custom_provider = keyForm.provider === 'Custom' ? key.provider : '';
  keyForm.pool_group = key.pool_group || 'default';
  keyForm.base_url = key.base_url || '';
  keyForm.proxy_url = key.proxy_url || '';
  keyForm.status = key.status || 'active';
  keyForm.keys = [{ key_name: key.key_name || '', key_value: '' }];
  showKeyModal.value = true;
};

const addKeyRow = () => keyForm.keys.push({ key_name: '', key_value: '' });
const removeKeyRow = (index: number) => {
  if (keyForm.keys.length === 1) return;
  keyForm.keys.splice(index, 1);
};

const saveKeys = async () => {
  try {
    const provider = providerName();
    if (!provider) return showToast('Custom provider name required', 'error');
    if (keyForm.provider === 'Custom' && !keyForm.base_url.trim()) return showToast('Custom provider requires Base URL', 'error');
    const rows = keyForm.keys.map(row => ({ key_name: row.key_name.trim(), key_value: row.key_value.trim() }));
    if (editingKeyId.value) {
      const row = rows[0];
      const payload: Record<string, string> = {
        provider,
        pool_group: keyForm.pool_group,
        key_name: row.key_name,
        base_url: keyForm.base_url,
        proxy_url: keyForm.proxy_url,
        status: keyForm.status,
      };
      if (row.key_value) payload.key_value = row.key_value;
      await api.patch(`/keys/${editingKeyId.value}`, payload);
      showToast('Key updated');
    } else {
      if (rows.some(row => !row.key_name || !row.key_value)) return showToast('Every key row needs a name and value', 'error');
      await api.post('/keys/create', { provider, pool_group: keyForm.pool_group, base_url: keyForm.base_url, proxy_url: keyForm.proxy_url, keys: rows });
      showToast('Keys saved');
    }
    showKeyModal.value = false;
    resetKeyForm();
    fetchKeys();
    fetchStats();
  } catch (error) {
    handleError(error);
  }
};

const openNewAccountModal = () => {
  resetAccountForm();
  showAccountModal.value = true;
};

const openEditAccountModal = (account: any) => {
  editingAccountId.value = account.id;
  accountForm.platform = account.platform || '';
  accountForm.account = account.account || '';
  accountForm.password = '';
  accountForm.url = account.url || '';
  accountForm.totp_secret = '';
  accountForm.favicon_url = account.favicon_url || '';
  showAccountModal.value = true;
};

const saveAccount = async () => {
  try {
    if (editingAccountId.value) {
      const payload: Record<string, string> = { platform: accountForm.platform, account: accountForm.account, url: accountForm.url, favicon_url: accountForm.favicon_url };
      if (accountForm.password) payload.password = accountForm.password;
      if (accountForm.totp_secret) payload.totp_secret = accountForm.totp_secret;
      await api.patch(`/accounts/${editingAccountId.value}`, payload);
      showToast('Account updated');
    } else {
      await api.post('/accounts/create', accountForm);
      showToast('Account saved');
    }
    showAccountModal.value = false;
    resetAccountForm();
    fetchAccounts();
  } catch (error) {
    handleError(error);
  }
};

const deleteItem = async (type: SecretType, id: number) => {
  if (!confirm('Are you sure?')) return;
  try {
    await api.delete(`/${type}/${id}`);
    showToast('Deleted');
    refreshActiveTab();
  } catch (error) {
    handleError(error);
  }
};

const decryptAndCopy = async (id: number, type: SecretType, clipId: string) => {
  try {
    const res = await api.get(`/${type}/${id}/decrypt`);
    const val = type === 'keys' ? res.data.key : res.data.password;
    await navigator.clipboard.writeText(val);
    copiedId.value = clipId;
    scheduleClipboardClear(val);
    showToast('Copied; clipboard clears in 30s');
    setTimeout(() => (copiedId.value = null), 2000);
  } catch (error) {
    handleError(error);
  }
};

const checkKeyQuota = async (id: number) => {
  try {
    const res = await api.post(`/keys/${id}/check-quota`);
    showToast(`Health check: ${res.data?.status || 'quota_error'}`);
    fetchKeys();
    fetchStats();
  } catch (error) {
    handleError(error);
  }
};

const listKeyModels = async (key: any) => {
  showModelsModal.value = true;
  modelListTitle.value = key.key_name || `Key #${key.id}`;
  loadingModels.value = true;
  modelListStatus.value = '';
  modelListError.value = '';
  keyModels.value = [];
  try {
    const res = await api.get(`/keys/${key.id}/models`);
    modelListStatus.value = res.data?.status || 'quota_error';
    modelListError.value = res.data?.error || '';
    keyModels.value = res.data?.models || [];
    showToast(`Models loaded: ${keyModels.value.length}`);
  } catch (error) {
    handleError(error);
  } finally {
    loadingModels.value = false;
  }
};

const generateInvite = async () => {
  try {
    await api.post('/admin/invites', { expires_in_hours: inviteExpiresInHours.value });
    showToast('Invite created');
    invitePage.value = 1;
    fetchInvites();
  } catch (error) {
    handleError(error);
  }
};

const fetchTOTP = async (id: number) => {
  try {
    const res = await api.get(`/accounts/${id}/totp`);
    totpCodes[id] = { code: res.data?.code || '', remaining: res.data?.remaining || 0 };
    showToast('TOTP generated');
  } catch (error) {
    handleError(error);
  }
};

const exportData = async (scope: ExportScope, format: ExportFormat) => {
  try {
    const prefix = scope === 'all' ? '' : `/${scope}`;
    const res = await api.get(`/export${prefix}/${format}`, { responseType: format === 'csv' ? 'blob' : 'json' });
    const blob = format === 'csv' ? new Blob([res.data], { type: 'text/csv' }) : new Blob([JSON.stringify(res.data, null, 2)], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `allinonekey-${scope}.${format}`;
    a.click();
    URL.revokeObjectURL(url);
    showToast(`Encrypted ${scope} ${format.toUpperCase()} exported`);
  } catch (error) {
    handleError(error);
  }
};

const onImportFileChange = (event: Event) => {
  const input = event.target as HTMLInputElement;
  importFile.value = input.files?.[0] || null;
};

const importData = async (scope: ExportScope) => {
  if (!importFile.value) return showToast('Choose an import file first', 'error');
  const file = importFile.value;
  const format: ExportFormat = file.name.toLowerCase().endsWith('.csv') ? 'csv' : 'json';
  try {
    if (format === 'json') {
      await api.post(`/import/${scope === 'all' ? '' : `${scope}/`}json`, JSON.parse(await file.text()));
    } else {
      await api.post(`/import/${scope === 'all' ? '' : `${scope}/`}csv`, await file.text(), { headers: { 'Content-Type': 'text/csv' } });
    }
    importFile.value = null;
    showToast('Import completed');
    refreshActiveTab();
  } catch (error) {
    handleError(error);
  }
};

const deleteInvite = async (id: number) => {
  if (!confirm('Delete this unused invite?')) return;
  try {
    await api.delete(`/admin/invites/${id}`);
    showToast('Invite deleted');
    if (invites.value.length === 1 && invitePage.value > 1) invitePage.value -= 1;
    fetchInvites();
  } catch (error) {
    handleError(error);
  }
};

const changeAuditPage = (nextPage: number) => {
  if (nextPage < 1 || nextPage > auditTotalPages.value || nextPage === auditPage.value) return;
  auditPage.value = nextPage;
  fetchLogs();
};

const changeInvitePage = (nextPage: number) => {
  if (nextPage < 1 || nextPage > inviteTotalPages.value || nextPage === invitePage.value) return;
  invitePage.value = nextPage;
  fetchInvites();
};

const handleLogout = async () => {
  if (clipboardClearTimer) clearTimeout(clipboardClearTimer);
  try {
    await navigator.clipboard.writeText('');
  } catch {
    // Clipboard access can be unavailable outside a focused secure context.
  }
  auth.logout();
  router.push('/login');
};
</script>

<template>
  <div class="min-h-screen vault-grid-bg text-slate-100 flex flex-col md:flex-row font-sans">
    <div v-if="toast" :class="['fixed top-4 right-4 z-[60] rounded-2xl border px-4 py-3 text-sm shadow-2xl backdrop-blur-xl', toast.type === 'error' ? 'bg-red-950/90 border-red-400/30 text-red-100' : 'bg-emerald-950/90 border-emerald-300/30 text-emerald-100']">
      {{ toast.message }}
    </div>

    <aside class="vault-surface w-full md:w-72 border-r border-white/10 flex md:flex-col md:fixed md:h-full z-20 rounded-none md:rounded-r-[1.5rem]">
      <div class="p-6 text-cyan-200 font-semibold text-xl tracking-[-0.03em] flex items-center gap-2"><ShieldCheck class="w-6 h-6" /> AllInOne</div>
      <nav class="flex-1 px-4 py-2 md:py-0 flex md:block gap-1 overflow-x-auto md:space-y-1">
        <button @click="activeTab = 'keys'" :class="['w-full flex items-center gap-3 px-4 py-3 rounded-2xl border transition-colors', activeTab === 'keys' ? 'bg-cyan-300/10 text-cyan-200 border-cyan-300/20 shadow-lg shadow-cyan-500/5' : 'text-slate-400 hover:text-cyan-100 hover:bg-white/[0.03] border-transparent']"><Key class="w-5 h-5" /> AI API Keys</button>
        <button @click="activeTab = 'accounts'" :class="['w-full flex items-center gap-3 px-4 py-3 rounded-2xl border transition-colors', activeTab === 'accounts' ? 'bg-cyan-300/10 text-cyan-200 border-cyan-300/20 shadow-lg shadow-cyan-500/5' : 'text-slate-400 hover:text-cyan-100 hover:bg-white/[0.03] border-transparent']"><Lock class="w-5 h-5" /> Accounts Vault</button>
        <button @click="activeTab = 'logs'" :class="['w-full flex items-center gap-3 px-4 py-3 rounded-2xl border transition-colors', activeTab === 'logs' ? 'bg-cyan-300/10 text-cyan-200 border-cyan-300/20 shadow-lg shadow-cyan-500/5' : 'text-slate-400 hover:text-cyan-100 hover:bg-white/[0.03] border-transparent']"><Activity class="w-5 h-5" /> Audit Logs</button>
        <div v-if="auth.role === 'admin'" class="pt-4 mt-4 border-t border-white/10"><p class="px-4 text-[10px] text-slate-500 uppercase tracking-[0.24em] mb-2">Admin Tools</p><button @click="activeTab = 'admin'" :class="['w-full flex items-center gap-3 px-4 py-3 rounded-2xl border transition-colors', activeTab === 'admin' ? 'bg-cyan-300/10 text-cyan-200 border-cyan-300/20 shadow-lg shadow-cyan-500/5' : 'text-slate-400 hover:text-cyan-100 hover:bg-white/[0.03] border-transparent']"><Ticket class="w-5 h-5" /> Invitations</button></div>
      </nav>
      <div class="p-4 border-t border-white/10"><button @click="handleLogout" class="w-full flex items-center gap-3 px-4 py-3 text-slate-400 hover:text-red-300 rounded-2xl hover:bg-red-400/5"><LogOut class="w-5 h-5" /> Logout</button></div>
    </aside>

    <main class="flex-1 md:ml-72 p-4 md:p-8 space-y-6">
      <div class="flex flex-col xl:flex-row gap-4 xl:justify-between xl:items-center">
        <div class="relative max-w-md w-full"><Search class="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-500" /><input v-model="searchQuery" placeholder="Search..." class="vault-input w-full rounded-2xl py-3 pl-10 pr-4 text-sm" /></div>
        <div class="flex flex-wrap gap-3">
          <button @click="exportData(activeExportScope, 'json')" class="vault-ghost-btn px-4 py-3 rounded-2xl text-sm flex items-center gap-2"><FileDown class="w-4 h-4"/>Export JSON</button>
          <button @click="exportData(activeExportScope, 'csv')" class="vault-ghost-btn px-4 py-3 rounded-2xl text-sm flex items-center gap-2"><FileDown class="w-4 h-4"/>Export CSV</button>
          <label v-if="isKeysTab || isAccountsTab" class="px-4 py-3 bg-gray-900 border border-gray-800 rounded-xl hover:text-blue-400 text-sm cursor-pointer flex items-center gap-2"><Upload class="w-4 h-4"/>Import<input type="file" accept=".json,.csv,application/json,text/csv" class="hidden" @change="onImportFileChange" /></label>
          <button v-if="importFile && (isKeysTab || isAccountsTab)" @click="importData(activeExportScope)" class="px-4 py-3 bg-emerald-500 rounded-2xl hover:bg-emerald-400 text-slate-950 text-sm font-semibold">Run Import</button>
          <button @click="refreshActiveTab" class="vault-ghost-btn p-3 rounded-2xl"><RefreshCw :class="['w-5 h-5', (loadingKeys || loadingStats || loadingAccounts || loadingLogs || loadingInvites) ? 'animate-spin' : '']"/></button>
          <button v-if="isKeysTab" @click="openNewKeyModal" class="vault-primary-btn px-6 py-3 rounded-2xl font-semibold flex items-center gap-2 text-sm"><Plus class="w-5 h-5"/> Add Keys</button>
          <button v-if="isAccountsTab" @click="openNewAccountModal" class="vault-primary-btn px-6 py-3 rounded-2xl font-semibold flex items-center gap-2 text-sm"><Plus class="w-5 h-5"/> New Account</button>
        </div>
      </div>

      <div v-if="activeTab === 'keys'" class="space-y-8">
        <div class="grid grid-cols-1 md:grid-cols-4 gap-5">
          <div class="vault-card p-6 rounded-2xl flex items-center gap-4"><div class="p-3 bg-cyan-300/10 text-cyan-200 rounded-2xl border border-cyan-300/10"><Key class="w-6 h-6"/></div><div><p class="text-xs text-gray-500 uppercase">Total Keys</p><p class="text-2xl font-bold">{{ keyStats.total }}</p></div></div>
          <div class="vault-card p-6 rounded-2xl flex items-center gap-4"><div class="p-3 bg-emerald-300/10 text-emerald-200 rounded-2xl border border-emerald-300/10"><CheckCircle2 class="w-6 h-6"/></div><div><p class="text-xs text-gray-500 uppercase">Active</p><p class="text-2xl font-bold">{{ keyStats.active }}</p></div></div>
          <div class="vault-card p-6 rounded-2xl flex items-center gap-4"><div class="p-3 bg-red-300/10 text-red-200 rounded-2xl border border-red-300/10"><AlertCircle class="w-6 h-6"/></div><div><p class="text-xs text-gray-500 uppercase">Issues</p><p class="text-2xl font-bold">{{ keyStats.error }}</p></div></div>
          <div class="vault-card p-6 rounded-2xl flex items-center gap-4"><div class="p-3 bg-amber-300/10 text-amber-200 rounded-2xl border border-amber-300/10"><RefreshCw :class="['w-6 h-6', loadingStats ? 'animate-spin' : '']"/></div><div><p class="text-xs text-gray-500 uppercase">Health</p><p class="text-2xl font-bold">{{ keyStats.active }}/{{ keyStats.total }}</p></div></div>
        </div>
        <div v-if="loadingKeys" class="text-center py-10 text-gray-500">Loading keys...</div>
        <div v-for="(pools, provider) in groupedKeys" :key="provider" class="space-y-4">
          <h3 class="text-xl font-bold text-gray-400 flex items-center gap-2"><Globe class="w-5 h-5"/> {{ provider }}</h3>
          <div v-for="(keysInGroup, groupName) in pools" :key="groupName" class="vault-card rounded-2xl overflow-x-auto">
            <div class="bg-white/[0.03] px-6 py-3 border-b border-white/10 flex justify-between items-center text-xs font-bold uppercase tracking-wider text-gray-500"><span>Pool: {{ groupName }}</span><span>{{ keysInGroup.length }} Keys</span></div>
            <table class="w-full min-w-[980px] text-left">
              <thead class="text-xs uppercase text-gray-500"><tr><th class="px-6 py-3">Key</th><th class="px-6 py-3">Secret</th><th class="px-6 py-3">Status</th><th class="px-6 py-3">Health Probe</th><th class="px-6 py-3">Endpoint</th><th class="px-6 py-3 text-right">Actions</th></tr></thead>
              <tbody class="divide-y divide-white/10">
                <tr v-for="k in keysInGroup" :key="k.id" class="hover:bg-white/[0.035] transition-colors">
                  <td class="px-6 py-4"><p class="text-sm font-semibold">{{ k.key_name }}</p><p class="text-xs text-gray-500">#{{ k.id }}</p></td>
                  <td class="px-6 py-4 font-mono text-xs text-gray-500"><button @click="decryptAndCopy(k.id, 'keys', 'k'+k.id)" class="flex items-center gap-2 hover:text-blue-400"><span>sk-••••••••••••</span><Check v-if="copiedId === 'k'+k.id" class="w-4 h-4 text-green-500"/><Copy v-else class="w-4 h-4"/></button></td>
                  <td class="px-6 py-4"><span :class="['px-2 py-0.5 rounded-full text-[10px] font-bold uppercase border', keyStatusClass(k.status)]">{{ k.status }}</span><p class="text-[11px] text-gray-600 mt-2">{{ formatDate(k.last_check) }}</p></td>
                  <td class="px-6 py-4 text-xs text-gray-400"><p class="text-gray-300">Availability only</p><p class="mt-1 text-[11px] text-gray-600">Real usage/billing is deferred.</p></td>
                  <td class="px-6 py-4 text-xs text-gray-500 max-w-xs"><p class="truncate">{{ k.base_url || 'Default endpoint' }}</p><p v-if="k.proxy_url" class="truncate text-yellow-500/80">Proxy: {{ k.proxy_url }}</p></td>
                  <td class="px-6 py-4 text-right"><button @click="checkKeyQuota(k.id)" class="mr-3 text-xs text-gray-500 hover:text-yellow-400">Check</button><button @click="listKeyModels(k)" class="mr-3 text-xs text-gray-500 hover:text-cyan-300">Models</button><button @click="openEditKeyModal(k)" class="mr-3 text-gray-600 hover:text-blue-400"><Edit3 class="w-4 h-4"/></button><button @click="deleteItem('keys', k.id)" class="text-gray-600 hover:text-red-400"><Trash2 class="w-4 h-4"/></button></td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
        <div v-if="!loadingKeys && keys.length === 0" class="text-center py-20 text-gray-600">No Keys found.</div>
      </div>

      <div v-if="activeTab === 'accounts'" class="space-y-6">
        <div v-if="loadingAccounts" class="text-center py-10 text-gray-500">Loading accounts...</div>
        <div class="grid grid-cols-1 lg:grid-cols-2 2xl:grid-cols-3 gap-5">
          <div v-for="a in accounts" :key="a.id" class="relative overflow-hidden vault-card p-5 rounded-2xl hover:border-cyan-300/35 transition-all">
            <div class="absolute inset-x-0 top-0 h-1 bg-gradient-to-r from-cyan-300/80 via-emerald-300/60 to-transparent"></div>
            <div class="flex justify-between gap-4 mb-5">
              <div class="min-w-0 flex items-center gap-3"><div class="w-10 h-10 rounded-xl bg-gray-800 border border-gray-700 flex items-center justify-center shrink-0"><img v-if="a.favicon_url" :src="a.favicon_url" class="w-6 h-6 rounded" referrerpolicy="no-referrer" /><Lock v-else class="w-5 h-5 text-green-400"/></div><div class="min-w-0"><h3 class="font-bold text-lg text-green-300 truncate">{{ a.platform }}</h3><p class="text-xs text-gray-500 truncate">{{ a.url || 'No URL' }}</p></div></div>
              <div class="flex gap-2 shrink-0"><a v-if="a.url" :href="a.url" target="_blank" class="text-gray-600 hover:text-blue-400"><ExternalLink class="w-4 h-4"/></a><button @click="openEditAccountModal(a)" class="text-gray-600 hover:text-blue-400"><Edit3 class="w-4 h-4"/></button><button @click="deleteItem('accounts', a.id)" class="text-gray-600 hover:text-red-400"><Trash2 class="w-4 h-4"/></button></div>
            </div>
            <div class="space-y-3">
              <div class="bg-gray-950/80 p-3 rounded-xl border border-gray-800 flex justify-between items-center gap-3"><span class="text-xs uppercase text-gray-600">Account</span><span class="text-xs font-mono text-gray-300 truncate">{{ a.account }}</span></div>
              <div class="bg-gray-950/80 p-3 rounded-xl border border-gray-800 flex justify-between items-center"><span class="font-mono text-xs text-gray-500">••••••••••••</span><button @click="decryptAndCopy(a.id, 'accounts', 'ap'+a.id)" class="text-gray-600 hover:text-blue-400"><Check v-if="copiedId === 'ap'+a.id" class="w-4 h-4 text-green-500"/><Copy v-else class="w-4 h-4"/></button></div>
              <div v-if="a.has_totp" class="bg-yellow-950/20 p-3 rounded-xl border border-yellow-900/40 flex justify-between items-center"><span class="font-mono text-lg tracking-widest text-yellow-300">{{ totpCodes[a.id]?.code || '••••••' }}</span><button @click="fetchTOTP(a.id)" class="text-xs text-gray-500 hover:text-yellow-400">TOTP {{ totpCodes[a.id]?.remaining ? `· ${totpCodes[a.id].remaining}s` : '' }}</button></div>
            </div>
          </div>
          <div v-if="!loadingAccounts && accounts.length === 0" class="col-span-full text-center py-20 text-gray-600">No Accounts found.</div>
        </div>
      </div>

      <div v-if="activeTab === 'logs'" class="space-y-4">
        <div class="vault-card rounded-2xl p-4 grid grid-cols-1 md:grid-cols-3 gap-3"><select v-model="auditFilters.action" class="vault-input rounded-2xl px-3 py-2 text-sm outline-none focus:border-blue-500"><option value="">All actions</option><option value="ADD_KEY">ADD_KEY</option><option value="UPDATE_KEY">UPDATE_KEY</option><option value="DELETE_KEY">DELETE_KEY</option><option value="CREATE_ACCOUNT">CREATE_ACCOUNT</option><option value="UPDATE_ACCOUNT">UPDATE_ACCOUNT</option><option value="EXPORT_KEYS_JSON">EXPORT_KEYS_JSON</option><option value="IMPORT_KEYS_CSV">IMPORT_KEYS_CSV</option></select><input v-model="auditFilters.keyword" placeholder="Search detail / IP..." class="md:col-span-2 vault-input rounded-2xl px-3 py-2 text-sm outline-none focus:border-blue-500" /></div>
        <div class="vault-card rounded-2xl overflow-hidden"><table class="w-full text-left"><thead class="bg-white/[0.035] text-xs uppercase text-slate-500"><tr><th class="px-6 py-4">Action</th><th class="px-6 py-4">IP</th><th class="px-6 py-4">Time</th></tr></thead><tbody class="divide-y divide-white/10"><tr v-if="loadingLogs"><td colspan="3" class="px-6 py-10 text-center text-gray-500">Loading logs...</td></tr><tr v-for="log in auditLogs" :key="log.id" class="hover:bg-gray-800/20"><td class="px-6 py-4"><span class="text-sm font-bold text-blue-400">{{ log.action }}</span><p class="text-xs text-gray-500">{{ log.detail }}</p></td><td class="px-6 py-4 text-xs text-gray-400">{{ log.ip }}</td><td class="px-6 py-4 text-xs text-gray-500">{{ new Date(log.created_at).toLocaleString() }}</td></tr><tr v-if="!loadingLogs && auditLogs.length === 0"><td colspan="3" class="px-6 py-10 text-center text-gray-600">No audit logs found.</td></tr></tbody></table><div class="flex items-center justify-between border-t border-white/10 px-6 py-4 text-sm text-gray-400"><span>Total {{ auditTotal }} logs · Page {{ auditPage }} / {{ auditTotalPages }}</span><div class="flex gap-2"><button @click="changeAuditPage(auditPage - 1)" :disabled="auditPage <= 1 || loadingLogs" class="px-3 py-1 rounded-lg border border-gray-800 disabled:opacity-40 hover:text-blue-400">Prev</button><button @click="changeAuditPage(auditPage + 1)" :disabled="auditPage >= auditTotalPages || loadingLogs" class="px-3 py-1 rounded-lg border border-gray-800 disabled:opacity-40 hover:text-blue-400">Next</button></div></div></div>
      </div>

      <div v-if="activeTab === 'admin'" class="space-y-6">
        <div class="flex flex-col md:flex-row gap-3 md:justify-between md:items-center"><h3 class="text-xl font-bold">Invitations</h3><div class="flex gap-2"><input v-model.number="inviteExpiresInHours" type="number" min="1" class="w-28 vault-input rounded-2xl px-3 py-2 text-sm" title="Expiry hours"/><button @click="generateInvite" class="bg-emerald-500 hover:bg-emerald-400 text-slate-950 px-4 py-2 rounded-lg text-sm font-bold">+ New Invite</button></div></div>
        <div class="vault-card rounded-2xl p-4"><select v-model="inviteFilters.status" class="vault-input rounded-2xl px-3 py-2 text-sm outline-none focus:border-blue-500"><option value="">All invites</option><option value="available">Available</option><option value="used">Used</option><option value="expired">Expired</option></select></div>
        <div class="vault-card rounded-2xl overflow-hidden"><table class="w-full text-left"><thead class="bg-white/[0.035] text-xs uppercase text-slate-500"><tr><th class="px-6 py-4">Code</th><th class="px-6 py-4">Status</th><th class="px-6 py-4">Expires</th><th class="px-6 py-4">Used By</th><th class="px-6 py-4 text-right">Actions</th></tr></thead><tbody class="divide-y divide-white/10"><tr v-if="loadingInvites"><td colspan="5" class="px-6 py-10 text-center text-gray-500">Loading invites...</td></tr><tr v-for="i in invites" :key="i.id"><td class="px-6 py-4 font-mono text-sm text-blue-300">{{ i.code }}</td><td class="px-6 py-4"><span :class="['px-2 py-0.5 rounded-full text-[10px] font-bold uppercase', i.is_used ? 'text-red-400 bg-red-400/10' : 'text-green-400 bg-green-400/10']">{{ i.is_used ? 'Used' : 'Available' }}</span></td><td class="px-6 py-4 text-xs text-gray-500">{{ i.expires_at ? new Date(i.expires_at).toLocaleString() : '-' }}</td><td class="px-6 py-4 text-xs text-gray-500">{{ i.used_by || '-' }}</td><td class="px-6 py-4 text-right"><button v-if="!i.is_used" @click="deleteInvite(i.id)" class="text-gray-600 hover:text-red-400"><Trash2 class="w-4 h-4"/></button></td></tr><tr v-if="!loadingInvites && invites.length === 0"><td colspan="5" class="px-6 py-10 text-center text-gray-600">No invites found.</td></tr></tbody></table><div class="flex items-center justify-between border-t border-white/10 px-6 py-4 text-sm text-gray-400"><span>Total {{ inviteTotal }} invites · Page {{ invitePage }} / {{ inviteTotalPages }}</span><div class="flex gap-2"><button @click="changeInvitePage(invitePage - 1)" :disabled="invitePage <= 1 || loadingInvites" class="px-3 py-1 rounded-lg border border-gray-800 disabled:opacity-40 hover:text-blue-400">Prev</button><button @click="changeInvitePage(invitePage + 1)" :disabled="invitePage >= inviteTotalPages || loadingInvites" class="px-3 py-1 rounded-lg border border-gray-800 disabled:opacity-40 hover:text-blue-400">Next</button></div></div></div>
      </div>
    </main>

    <div v-if="showKeyModal" class="fixed inset-0 bg-black/80 backdrop-blur-sm flex items-center justify-center p-4 z-50">
      <div class="vault-card rounded-2xl w-full max-w-3xl max-h-[90vh] overflow-y-auto">
        <div class="p-6 border-b border-white/10 flex justify-between items-center"><h3 class="text-xl font-bold">{{ keyModalTitle }}</h3><button @click="showKeyModal = false"><X class="w-6 h-6"/></button></div>
        <div class="p-6 space-y-4">
          <div class="grid grid-cols-1 md:grid-cols-2 gap-4"><div><label class="text-xs font-bold text-gray-500 uppercase">Provider</label><select v-model="keyForm.provider" class="w-full vault-input rounded-xl p-2.5 text-sm"><option>OpenAI</option><option>DeepSeek</option><option>Anthropic</option><option>Gemini</option><option>Custom</option></select></div><div><label class="text-xs font-bold text-gray-500 uppercase">Pool Group</label><input v-model="keyForm.pool_group" class="w-full vault-input rounded-xl p-2.5 text-sm" /></div></div>
          <div v-if="keyForm.provider === 'Custom'"><label class="text-xs font-bold text-gray-500 uppercase">Custom Provider Name</label><input v-model="keyForm.custom_provider" placeholder="one-api / new-api / company-relay" class="w-full vault-input rounded-xl p-2.5 text-sm" /></div>
          <div v-for="(row, idx) in keyForm.keys" :key="idx" class="grid grid-cols-1 md:grid-cols-[1fr_1.4fr_auto] gap-3 bg-slate-950/45 border border-white/10 rounded-2xl p-3"><input v-model="row.key_name" placeholder="Custom key name" class="vault-input rounded-xl p-2.5 text-sm" /><input v-model="row.key_value" :placeholder="editingKeyId ? 'Leave empty to keep current key value' : 'API key value'" class="vault-input rounded-xl p-2.5 text-sm font-mono" /><button v-if="!editingKeyId" @click="removeKeyRow(idx)" class="text-gray-500 hover:text-red-400"><Trash2 class="w-4 h-4"/></button></div>
          <button v-if="!editingKeyId" @click="addKeyRow" class="px-3 py-2 border border-dashed border-slate-600 rounded-xl text-sm text-gray-400 hover:text-blue-400 hover:border-blue-500">+ Add another row</button>
          <div class="grid grid-cols-1 md:grid-cols-2 gap-4"><div><label class="text-xs font-bold text-gray-500 uppercase">Base URL</label><input v-model="keyForm.base_url" :placeholder="keyForm.provider === 'Custom' ? 'https://relay.example.com' : 'Optional provider override'" class="w-full vault-input rounded-xl p-2.5 text-sm" /></div><div><label class="text-xs font-bold text-gray-500 uppercase">Proxy URL</label><input v-model="keyForm.proxy_url" placeholder="Optional: http://127.0.0.1:7890" class="w-full vault-input rounded-xl p-2.5 text-sm" /></div></div>
          <div v-if="editingKeyId"><label class="text-xs font-bold text-gray-500 uppercase">Status</label><input v-model="keyForm.status" class="w-full vault-input rounded-xl p-2.5 text-sm" /></div>
        </div>
        <div class="p-6 bg-white/[0.035] flex justify-end gap-3"><button @click="saveKeys" class="vault-primary-btn px-6 py-2 rounded-xl font-semibold">Save</button></div>
      </div>
    </div>

    <div v-if="showModelsModal" class="fixed inset-0 bg-black/80 backdrop-blur-sm flex items-center justify-center p-4 z-50">
      <div class="vault-card rounded-2xl w-full max-w-2xl max-h-[90vh] overflow-hidden shadow-2xl">
        <div class="p-6 border-b border-white/10 flex justify-between items-center"><div><h3 class="text-xl font-bold flex items-center gap-2"><ServerCog class="w-5 h-5 text-cyan-300"/> {{ modelListTitle }}</h3><p class="text-xs text-gray-500 mt-1">Provider model list · status: {{ modelListStatus || 'loading' }}</p></div><button @click="showModelsModal = false"><X class="w-6 h-6"/></button></div>
        <div class="p-6 max-h-[65vh] overflow-y-auto">
          <div v-if="loadingModels" class="py-12 text-center text-gray-500">Loading models...</div>
          <div v-else-if="modelListError" class="rounded-2xl border border-red-400/20 bg-red-950/30 p-4 text-sm text-red-100">{{ modelListError }}</div>
          <div v-else-if="keyModels.length === 0" class="py-12 text-center text-gray-600">No models returned.</div>
          <div v-else class="grid grid-cols-1 md:grid-cols-2 gap-3">
            <div v-for="m in keyModels" :key="m.id || m.name" class="rounded-2xl border border-white/10 bg-slate-950/45 p-4">
              <p class="font-mono text-sm text-cyan-100 break-all">{{ m.id || m.name }}</p>
              <p v-if="m.owned_by" class="text-xs text-gray-500 mt-2">{{ m.owned_by }}</p>
            </div>
          </div>
        </div>
      </div>
    </div>

    <div v-if="showAccountModal" class="fixed inset-0 bg-black/80 backdrop-blur-sm flex items-center justify-center p-4 z-50">
      <div class="vault-card rounded-2xl w-full max-w-md shadow-2xl">
        <div class="p-6 border-b border-white/10 flex justify-between items-center"><h3 class="text-xl font-bold">{{ accountModalTitle }}</h3><button @click="showAccountModal = false"><X class="w-6 h-6"/></button></div>
        <div class="p-6 space-y-4"><div><label class="text-xs font-bold text-gray-500 uppercase">Platform</label><input v-model="accountForm.platform" class="w-full vault-input rounded-xl p-2.5 text-sm" /></div><div><label class="text-xs font-bold text-gray-500 uppercase">Account</label><input v-model="accountForm.account" class="w-full vault-input rounded-xl p-2.5 text-sm" /></div><div><label class="text-xs font-bold text-gray-500 uppercase">Password</label><input v-model="accountForm.password" type="password" :placeholder="editingAccountId ? 'Leave empty to keep current password' : ''" class="w-full vault-input rounded-xl p-2.5 text-sm" /></div><div><label class="text-xs font-bold text-gray-500 uppercase">URL</label><input v-model="accountForm.url" class="w-full vault-input rounded-xl p-2.5 text-sm" /></div><div><label class="text-xs font-bold text-gray-500 uppercase">TOTP Secret</label><input v-model="accountForm.totp_secret" placeholder="Optional raw base32 secret" class="w-full vault-input rounded-xl p-2.5 text-sm" /></div><div><label class="text-xs font-bold text-gray-500 uppercase">Favicon URL</label><input v-model="accountForm.favicon_url" placeholder="Optional override" class="w-full vault-input rounded-xl p-2.5 text-sm" /></div></div>
        <div class="p-6 bg-white/[0.035] flex justify-end gap-3"><button @click="saveAccount" class="vault-primary-btn px-6 py-2 rounded-xl font-semibold">Save Account</button></div>
      </div>
    </div>
  </div>
</template>
