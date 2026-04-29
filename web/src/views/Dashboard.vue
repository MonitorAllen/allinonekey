<script setup lang="ts">
import { ref, onMounted, reactive, computed, watch } from 'vue';
import { useRouter } from 'vue-router';
import api, { getApiErrorMessage } from '../api';
import { useAuthStore } from '../store/auth';
import { Key, Lock, LogOut, Plus, ShieldCheck, RefreshCw, X, Globe, Trash2, Copy, Check, ExternalLink, Activity, Search, AlertCircle, CheckCircle2, Ticket } from 'lucide-vue-next';

const router = useRouter();
const auth = useAuthStore();
const activeTab = ref('keys');
const keys = ref<any[]>([]);
const accounts = ref<any[]>([]);
const invites = ref<any[]>([]);
const auditLogs = ref<any[]>([]);
const keyStats = ref({ total: 0, active: 0, error: 0, balance: 0 });
const searchQuery = ref('');
const showBulkAddModal = ref(false);
const showAccountModal = ref(false);
const copiedId = ref<string | null>(null);
const toast = ref<{ message: string; type: 'success' | 'error' } | null>(null);
const CLIPBOARD_CLEAR_DELAY_MS = 30000;
let clipboardClearTimer: ReturnType<typeof setTimeout> | null = null;

const loadingStats = ref(false);
const loadingKeys = ref(false);
const loadingAccounts = ref(false);
const loadingLogs = ref(false);
const loadingInvites = ref(false);

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

const newKeyForm = reactive({ provider: 'OpenAI', custom_provider: '', pool_group: 'default', base_url: '', proxy_url: '', raw_keys: '' });
const newAccountForm = reactive({ platform: '', account: '', password: '', url: '', totp_secret: '', favicon_url: '' });
const inviteExpiresInHours = ref(168);
const totpCodes = reactive<Record<number, { code: string; remaining: number }>>({});

const showToast = (message: string, type: 'success' | 'error' = 'success') => {
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
      // Some browsers deny clipboard reads. In that case, avoid clearing blindly.
    } finally {
      clipboardClearTimer = null;
    }
  }, CLIPBOARD_CLEAR_DELAY_MS);
};

// Fetchers
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
      params: {
        page: auditPage.value,
        page_size: auditPageSize.value,
        action: auditFilters.action || undefined,
        keyword: auditFilters.keyword || undefined,
      },
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
    const res = await api.get('/admin/invites', {
      params: {
        page: invitePage.value,
        page_size: invitePageSize.value,
        status: inviteFilters.status || undefined,
      },
    });
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

// Dispatcher
const refreshActiveTab = () => {
  if (activeTab.value === 'keys') {
    fetchKeys();
    fetchStats();
  } else if (activeTab.value === 'accounts') fetchAccounts();
  else if (activeTab.value === 'logs') fetchLogs();
  else if (activeTab.value === 'admin') fetchInvites();
};

onMounted(() => {
  refreshActiveTab();
});

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

// Grouping
const groupedKeys = computed(() => {
  const groups: any = {};
  keys.value.forEach(k => {
    if (!groups[k.provider]) groups[k.provider] = {};
    const gName = k.pool_group || 'default';
    if (!groups[k.provider][gName]) groups[k.provider][gName] = [];
    groups[k.provider][gName].push(k);
  });
  return groups;
});

// Actions
const handleLogout = async () => {
  if (clipboardClearTimer) {
    clearTimeout(clipboardClearTimer);
    clipboardClearTimer = null;
  }
  try {
    await navigator.clipboard.writeText('');
  } catch {
    // Clipboard access can be unavailable outside a focused secure context.
  }
  auth.logout();
  router.push('/login');
};

const handleBulkAdd = async () => {
  try {
    const provider = newKeyForm.provider === 'Custom' ? newKeyForm.custom_provider.trim() : newKeyForm.provider;
    if (!provider) {
      showToast('Custom provider name required', 'error');
      return;
    }
    if (newKeyForm.provider === 'Custom' && !newKeyForm.base_url.trim()) {
      showToast('Custom provider requires Base URL', 'error');
      return;
    }
    await api.post('/keys/bulk', {
      provider,
      pool_group: newKeyForm.pool_group,
      base_url: newKeyForm.base_url,
      proxy_url: newKeyForm.proxy_url,
      raw_keys: newKeyForm.raw_keys,
    });
    showBulkAddModal.value = false;
    newKeyForm.raw_keys = '';
    showToast('Keys imported');
    fetchKeys();
    fetchStats();
  } catch (error) {
    handleError(error);
  }
};

const handleAddAccount = async () => {
  try {
    await api.post('/accounts/create', newAccountForm);
    showAccountModal.value = false;
    newAccountForm.platform = '';
    newAccountForm.account = '';
    newAccountForm.password = '';
    newAccountForm.totp_secret = '';
    newAccountForm.favicon_url = '';
    showToast('Account saved');
    fetchAccounts();
  } catch (error) {
    handleError(error);
  }
};

const deleteItem = async (type: string, id: number) => {
  if (!confirm('Are you sure?')) return;
  try {
    await api.delete(`/${type}/${id}`);
    showToast('Deleted');
    refreshActiveTab();
  } catch (error) {
    handleError(error);
  }
};

const decryptAndCopy = async (id: number, type: 'keys' | 'accounts', clipId: string) => {
  try {
    const res = await api.get(`/${type}/${id}/decrypt`);
    const val = type === 'keys' ? res.data.key : res.data.password;
    await navigator.clipboard.writeText(val);
    copiedId.value = clipId;
    scheduleClipboardClear(val);
    showToast('Copied; clipboard clears in 30s');
    setTimeout(() => {
      copiedId.value = null;
    }, 2000);
  } catch (error) {
    handleError(error);
  }
};

const checkKeyQuota = async (id: number) => {
  try {
    const res = await api.post(`/keys/${id}/check-quota`);
    const status = res.data?.status || 'quota_error';
    showToast(`Quota check: ${status}`);
    fetchKeys();
    fetchStats();
  } catch (error) {
    handleError(error);
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

const exportData = async (format: 'json' | 'csv') => {
  try {
    const res = await api.get(`/export/${format}`, { responseType: format === 'csv' ? 'blob' : 'json' });
    const blob = format === 'csv'
      ? new Blob([res.data], { type: 'text/csv' })
      : new Blob([JSON.stringify(res.data, null, 2)], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `allinonekey-export.${format}`;
    a.click();
    URL.revokeObjectURL(url);
    showToast(`Encrypted ${format.toUpperCase()} exported`);
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
</script>

<template>
  <div class="min-h-screen bg-gray-950 text-gray-100 flex flex-col md:flex-row font-sans">
    <div v-if="toast" :class="['fixed top-4 right-4 z-[60] rounded-xl border px-4 py-3 text-sm shadow-2xl', toast.type === 'error' ? 'bg-red-950 border-red-800 text-red-100' : 'bg-green-950 border-green-800 text-green-100']">
      {{ toast.message }}
    </div>

    <aside class="w-full md:w-64 bg-gray-900 border-r border-gray-800 flex md:flex-col md:fixed md:h-full z-20">
      <div class="p-6 text-blue-400 font-bold text-xl flex items-center gap-2">
        <ShieldCheck class="w-6 h-6" /> AllInOne
      </div>
      <nav class="flex-1 px-4 py-2 md:py-0 flex md:block gap-1 overflow-x-auto md:space-y-1">
        <button @click="activeTab = 'keys'" :class="['w-full flex items-center gap-3 px-4 py-3 rounded-lg', activeTab === 'keys' ? 'bg-blue-600/20 text-blue-400' : 'text-gray-400']">
          <Key class="w-5 h-5" /> AI API Keys
        </button>
        <button @click="activeTab = 'accounts'" :class="['w-full flex items-center gap-3 px-4 py-3 rounded-lg', activeTab === 'accounts' ? 'bg-blue-600/20 text-blue-400' : 'text-gray-400']">
          <Lock class="w-5 h-5" /> Accounts Vault
        </button>
        <button @click="activeTab = 'logs'" :class="['w-full flex items-center gap-3 px-4 py-3 rounded-lg', activeTab === 'logs' ? 'bg-blue-600/20 text-blue-400' : 'text-gray-400']">
          <Activity class="w-5 h-5" /> Audit Logs
        </button>
        <div v-if="auth.role === 'admin'" class="pt-4 mt-4 border-t border-gray-800">
          <p class="px-4 text-[10px] text-gray-500 uppercase mb-2">Admin Tools</p>
          <button @click="activeTab = 'admin'" :class="['w-full flex items-center gap-3 px-4 py-3 rounded-lg', activeTab === 'admin' ? 'bg-blue-600/20 text-blue-400' : 'text-gray-400']">
            <Ticket class="w-5 h-5" /> Invitations
          </button>
        </div>
      </nav>
      <div class="p-4 border-t border-gray-800">
        <button @click="handleLogout" class="w-full flex items-center gap-3 px-4 py-3 text-gray-400 hover:text-red-400">
          <LogOut class="w-5 h-5" /> Logout
        </button>
      </div>
    </aside>

    <main class="flex-1 md:ml-64 p-4 md:p-8">
      <!-- Header -->
      <div class="flex flex-col md:flex-row gap-4 md:justify-between md:items-center mb-8">
        <div class="relative max-w-md w-full">
          <Search class="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-500" />
          <input v-model="searchQuery" placeholder="Search..." class="w-full bg-gray-900 border-gray-800 rounded-xl py-3 pl-10 pr-4 text-sm outline-none border focus:border-blue-500" />
        </div>
        <div class="flex flex-wrap gap-3">
          <button @click="exportData('json')" class="px-4 py-3 bg-gray-900 border border-gray-800 rounded-xl hover:text-blue-400 text-sm">Export JSON</button>
          <button @click="exportData('csv')" class="px-4 py-3 bg-gray-900 border border-gray-800 rounded-xl hover:text-blue-400 text-sm">Export CSV</button>
          <button @click="refreshActiveTab" class="p-3 bg-gray-900 border border-gray-800 rounded-xl hover:text-blue-400"><RefreshCw :class="['w-5 h-5', (loadingKeys || loadingStats || loadingAccounts || loadingLogs || loadingInvites) ? 'animate-spin' : '']"/></button>
          <button v-if="activeTab === 'keys'" @click="showBulkAddModal = true" class="bg-blue-600 hover:bg-blue-700 px-6 py-3 rounded-xl font-bold flex items-center gap-2 text-sm"><Plus class="w-5 h-5"/> Bulk Add Keys</button>
          <button v-if="activeTab === 'accounts'" @click="showAccountModal = true" class="bg-blue-600 hover:bg-blue-700 px-6 py-3 rounded-xl font-bold flex items-center gap-2 text-sm"><Plus class="w-5 h-5"/> New Account</button>
        </div>
      </div>

      <!-- Keys View -->
      <div v-if="activeTab === 'keys'" class="space-y-8">
        <div class="grid grid-cols-1 md:grid-cols-4 gap-6">
          <div class="bg-gray-900 p-6 rounded-xl border border-gray-800 flex items-center gap-4">
            <div class="p-3 bg-blue-600/10 text-blue-400 rounded-lg"><Key class="w-6 h-6"/></div>
            <div><p class="text-xs text-gray-500 uppercase">Total Keys</p><p class="text-2xl font-bold">{{ keyStats.total }}</p></div>
          </div>
          <div class="bg-gray-900 p-6 rounded-xl border border-gray-800 flex items-center gap-4">
            <div class="p-3 bg-green-600/10 text-green-400 rounded-lg"><CheckCircle2 class="w-6 h-6"/></div>
            <div><p class="text-xs text-gray-500 uppercase">Active</p><p class="text-2xl font-bold">{{ keyStats.active }}</p></div>
          </div>
          <div class="bg-gray-900 p-6 rounded-xl border border-gray-800 flex items-center gap-4">
            <div class="p-3 bg-red-600/10 text-red-400 rounded-lg"><AlertCircle class="w-6 h-6"/></div>
            <div><p class="text-xs text-gray-500 uppercase">Issues</p><p class="text-2xl font-bold">{{ keyStats.error }}</p></div>
          </div>
          <div class="bg-gray-900 p-6 rounded-xl border border-gray-800 flex items-center gap-4">
            <div class="p-3 bg-yellow-600/10 text-yellow-400 rounded-lg"><RefreshCw :class="['w-6 h-6', loadingStats ? 'animate-spin' : '']"/></div>
            <div><p class="text-xs text-gray-500 uppercase">Quota Probe</p><p class="text-sm font-bold text-gray-400">Models endpoint health</p></div>
          </div>
        </div>

        <div v-if="loadingKeys" class="text-center py-10 text-gray-500">Loading keys...</div>
        <div v-for="(pools, provider) in groupedKeys" :key="provider" class="space-y-4">
          <h3 class="text-xl font-bold text-gray-400 flex items-center gap-2"><Globe class="w-5 h-5"/> {{ provider }}</h3>
          <div v-for="(keysInGroup, groupName) in pools" :key="groupName" class="bg-gray-900 rounded-2xl border border-gray-800 overflow-hidden">
            <div class="bg-gray-800/30 px-6 py-3 border-b border-gray-800 flex justify-between items-center text-xs font-bold uppercase tracking-wider text-gray-500">
              <span>Pool: {{ groupName }}</span>
              <span>{{ keysInGroup.length }} Keys</span>
            </div>
            <table class="w-full text-left">
              <tbody class="divide-y divide-gray-800">
                <tr v-for="k in keysInGroup" :key="k.id" class="hover:bg-gray-800/20 transition-colors">
                  <td class="px-6 py-4 text-sm font-medium">{{ k.key_name }}</td>
                  <td class="px-6 py-4 font-mono text-xs text-gray-500">
                    <div class="flex items-center gap-2">
                      <span>sk-••••••••••••</span>
                      <button @click="decryptAndCopy(k.id, 'keys', 'k'+k.id)" class="hover:text-blue-400">
                        <Check v-if="copiedId === 'k'+k.id" class="w-4 h-4 text-green-500"/>
                        <Copy v-else class="w-4 h-4"/>
                      </button>
                    </div>
                  </td>
                  <td class="px-6 py-4">
                    <span :class="['px-2 py-0.5 rounded-full text-[10px] font-bold uppercase', k.status === 'active' ? 'text-green-400 bg-green-400/10' : 'text-red-400 bg-red-400/10']">{{ k.status }}</span>
                  </td>
                  <td class="px-6 py-4 text-right">
                    <button @click="checkKeyQuota(k.id)" class="mr-3 text-xs text-gray-500 hover:text-yellow-400">Check</button>
                    <button @click="deleteItem('keys', k.id)" class="text-gray-600 hover:text-red-400"><Trash2 class="w-4 h-4"/></button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
        <div v-if="!loadingKeys && keys.length === 0" class="text-center py-20 text-gray-600">No Keys found.</div>
      </div>

      <!-- Accounts View -->
      <div v-if="activeTab === 'accounts'">
        <div v-if="loadingAccounts" class="text-center py-10 text-gray-500">Loading accounts...</div>
        <div class="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-6">
          <div v-for="a in accounts" :key="a.id" class="bg-gray-900 p-6 rounded-2xl border border-gray-800 group hover:border-blue-500/30 transition-all">
            <div class="flex justify-between mb-4">
              <div class="flex items-center gap-2"><img v-if="a.favicon_url" :src="a.favicon_url" class="w-5 h-5 rounded" referrerpolicy="no-referrer" /><h3 class="font-bold text-lg text-green-400">{{ a.platform }}</h3></div>
              <div class="flex gap-2">
                <a v-if="a.url" :href="a.url" target="_blank" class="text-gray-600 hover:text-blue-400"><ExternalLink class="w-4 h-4"/></a>
                <button @click="deleteItem('accounts', a.id)" class="text-gray-600 hover:text-red-400"><Trash2 class="w-4 h-4"/></button>
              </div>
            </div>
            <div class="space-y-3">
              <div class="bg-gray-950 p-3 rounded-xl border border-gray-800 flex justify-between items-center">
                <span class="text-xs font-mono text-gray-300">{{ a.account }}</span>
                <button @click="decryptAndCopy(a.id, 'accounts', 'ap'+a.id)" class="text-gray-600 hover:text-blue-400">
                  <Check v-if="copiedId === 'ap'+a.id" class="w-4 h-4 text-green-500"/>
                  <Copy v-else class="w-4 h-4"/>
                </button>
              </div>
              <div v-if="a.has_totp" class="bg-gray-950 p-3 rounded-xl border border-gray-800 flex justify-between items-center">
                <span class="font-mono text-lg tracking-widest text-yellow-300">{{ totpCodes[a.id]?.code || '••••••' }}</span>
                <button @click="fetchTOTP(a.id)" class="text-xs text-gray-500 hover:text-yellow-400">TOTP {{ totpCodes[a.id]?.remaining ? `· ${totpCodes[a.id].remaining}s` : '' }}</button>
              </div>
            </div>
          </div>
          <div v-if="!loadingAccounts && accounts.length === 0" class="col-span-full text-center py-20 text-gray-600">No Accounts found.</div>
        </div>
      </div>

      <!-- Logs View -->
      <div v-if="activeTab === 'logs'" class="space-y-4">
        <div class="bg-gray-900 rounded-2xl border border-gray-800 p-4 grid grid-cols-1 md:grid-cols-3 gap-3">
          <select v-model="auditFilters.action" class="bg-gray-950 border border-gray-800 rounded-xl px-3 py-2 text-sm outline-none focus:border-blue-500">
            <option value="">All actions</option>
            <option value="BULK_ADD_KEY">BULK_ADD_KEY</option>
            <option value="UPDATE_KEY">UPDATE_KEY</option>
            <option value="DELETE_KEY">DELETE_KEY</option>
            <option value="DECRYPT_KEY">DECRYPT_KEY</option>
            <option value="CREATE_ACCOUNT">CREATE_ACCOUNT</option>
            <option value="UPDATE_ACCOUNT">UPDATE_ACCOUNT</option>
            <option value="DELETE_ACCOUNT">DELETE_ACCOUNT</option>
            <option value="DECRYPT_ACCOUNT">DECRYPT_ACCOUNT</option>
            <option value="GENERATE_TOTP">GENERATE_TOTP</option>
            <option value="EXPORT_DATA_CSV">EXPORT_DATA_CSV</option>
            <option value="IMPORT_DATA_JSON">IMPORT_DATA_JSON</option>
          </select>
          <input v-model="auditFilters.keyword" placeholder="Search detail / IP..." class="md:col-span-2 bg-gray-950 border border-gray-800 rounded-xl px-3 py-2 text-sm outline-none focus:border-blue-500" />
        </div>
        <div class="bg-gray-900 rounded-2xl border border-gray-800 overflow-hidden">
          <table class="w-full text-left">
            <thead class="bg-gray-800/50 text-xs uppercase text-gray-500">
              <tr><th class="px-6 py-4">Action</th><th class="px-6 py-4">IP</th><th class="px-6 py-4">Time</th></tr>
            </thead>
            <tbody class="divide-y divide-gray-800">
              <tr v-if="loadingLogs"><td colspan="3" class="px-6 py-10 text-center text-gray-500">Loading logs...</td></tr>
              <tr v-for="log in auditLogs" :key="log.id" class="hover:bg-gray-800/20">
                <td class="px-6 py-4">
                  <span class="text-sm font-bold text-blue-400">{{ log.action }}</span>
                  <p class="text-xs text-gray-500">{{ log.detail }}</p>
                </td>
                <td class="px-6 py-4 text-xs text-gray-400">{{ log.ip }}</td>
                <td class="px-6 py-4 text-xs text-gray-500">{{ new Date(log.created_at).toLocaleString() }}</td>
              </tr>
              <tr v-if="!loadingLogs && auditLogs.length === 0"><td colspan="3" class="px-6 py-10 text-center text-gray-600">No audit logs found.</td></tr>
            </tbody>
          </table>
          <div class="flex items-center justify-between border-t border-gray-800 px-6 py-4 text-sm text-gray-400">
            <span>Total {{ auditTotal }} logs · Page {{ auditPage }} / {{ auditTotalPages }}</span>
            <div class="flex gap-2">
              <button @click="changeAuditPage(auditPage - 1)" :disabled="auditPage <= 1 || loadingLogs" class="px-3 py-1 rounded-lg border border-gray-800 disabled:opacity-40 hover:text-blue-400">Prev</button>
              <button @click="changeAuditPage(auditPage + 1)" :disabled="auditPage >= auditTotalPages || loadingLogs" class="px-3 py-1 rounded-lg border border-gray-800 disabled:opacity-40 hover:text-blue-400">Next</button>
            </div>
          </div>
        </div>
      </div>

      <!-- Admin View -->
      <div v-if="activeTab === 'admin'" class="space-y-6">
        <div class="flex flex-col md:flex-row gap-3 md:justify-between md:items-center"><h3 class="text-xl font-bold">Invitations</h3><div class="flex gap-2"><input v-model.number="inviteExpiresInHours" type="number" min="1" class="w-28 bg-gray-900 border border-gray-800 rounded-lg px-3 py-2 text-sm" title="Expiry hours"/><button @click="generateInvite" class="bg-green-600 hover:bg-green-700 px-4 py-2 rounded-lg text-sm font-bold">+ New Invite</button></div></div>
        <div class="bg-gray-900 rounded-2xl border border-gray-800 p-4">
          <select v-model="inviteFilters.status" class="bg-gray-950 border border-gray-800 rounded-xl px-3 py-2 text-sm outline-none focus:border-blue-500">
            <option value="">All invites</option>
            <option value="available">Available</option>
            <option value="used">Used</option>
            <option value="expired">Expired</option>
          </select>
        </div>
        <div class="bg-gray-900 rounded-2xl border border-gray-800 overflow-hidden">
          <table class="w-full text-left">
            <thead class="bg-gray-800/50 text-xs uppercase text-gray-500"><tr><th class="px-6 py-4">Code</th><th class="px-6 py-4">Status</th><th class="px-6 py-4">Expires</th><th class="px-6 py-4">Used By</th><th class="px-6 py-4 text-right">Actions</th></tr></thead>
            <tbody class="divide-y divide-gray-800">
              <tr v-if="loadingInvites"><td colspan="5" class="px-6 py-10 text-center text-gray-500">Loading invites...</td></tr>
              <tr v-for="i in invites" :key="i.id">
                <td class="px-6 py-4 font-mono text-sm text-blue-300">{{ i.code }}</td>
                <td class="px-6 py-4"><span :class="['px-2 py-0.5 rounded-full text-[10px] font-bold uppercase', i.is_used ? 'text-red-400 bg-red-400/10' : 'text-green-400 bg-green-400/10']">{{ i.is_used ? 'Used' : 'Available' }}</span></td>
                <td class="px-6 py-4 text-xs text-gray-500">{{ i.expires_at ? new Date(i.expires_at).toLocaleString() : '-' }}</td>
                <td class="px-6 py-4 text-xs text-gray-500">{{ i.used_by || '-' }}</td>
                <td class="px-6 py-4 text-right"><button v-if="!i.is_used" @click="deleteInvite(i.id)" class="text-gray-600 hover:text-red-400"><Trash2 class="w-4 h-4"/></button></td>
              </tr>
              <tr v-if="!loadingInvites && invites.length === 0"><td colspan="5" class="px-6 py-10 text-center text-gray-600">No invites found.</td></tr>
            </tbody>
          </table>
          <div class="flex items-center justify-between border-t border-gray-800 px-6 py-4 text-sm text-gray-400">
            <span>Total {{ inviteTotal }} invites · Page {{ invitePage }} / {{ inviteTotalPages }}</span>
            <div class="flex gap-2">
              <button @click="changeInvitePage(invitePage - 1)" :disabled="invitePage <= 1 || loadingInvites" class="px-3 py-1 rounded-lg border border-gray-800 disabled:opacity-40 hover:text-blue-400">Prev</button>
              <button @click="changeInvitePage(invitePage + 1)" :disabled="invitePage >= inviteTotalPages || loadingInvites" class="px-3 py-1 rounded-lg border border-gray-800 disabled:opacity-40 hover:text-blue-400">Next</button>
            </div>
          </div>
        </div>
      </div>
    </main>

    <!-- Modals -->
    <div v-if="showBulkAddModal" class="fixed inset-0 bg-black/80 backdrop-blur-sm flex items-center justify-center p-4 z-50">
      <div class="bg-gray-900 border border-gray-800 rounded-2xl w-full max-w-lg">
        <div class="p-6 border-b border-gray-800 flex justify-between items-center"><h3 class="text-xl font-bold">Bulk Add Keys</h3><button @click="showBulkAddModal = false"><X class="w-6 h-6"/></button></div>
        <div class="p-6 space-y-4">
          <div class="grid grid-cols-2 gap-4">
            <div><label class="text-xs font-bold text-gray-500 uppercase">Provider</label><select v-model="newKeyForm.provider" class="w-full bg-gray-800 border-gray-700 rounded-lg p-2.5 text-sm"><option>OpenAI</option><option>DeepSeek</option><option>Anthropic</option><option>Gemini</option><option>Custom</option></select></div>
            <div><label class="text-xs font-bold text-gray-500 uppercase">Pool Group</label><input v-model="newKeyForm.pool_group" class="w-full bg-gray-800 border-gray-700 rounded-lg p-2.5 text-sm" /></div>
          </div>
          <div v-if="newKeyForm.provider === 'Custom'"><label class="text-xs font-bold text-gray-500 uppercase">Custom Provider Name</label><input v-model="newKeyForm.custom_provider" placeholder="e.g. one-api / new-api / company-relay" class="w-full bg-gray-800 border-gray-700 rounded-lg p-2.5 text-sm" /></div>
          <div><label class="text-xs font-bold text-gray-500 uppercase">API Keys (One per line)</label><textarea v-model="newKeyForm.raw_keys" rows="6" class="w-full bg-gray-800 border-gray-700 rounded-lg p-2.5 text-sm font-mono"></textarea></div>
          <div><label class="text-xs font-bold text-gray-500 uppercase">Base URL</label><input v-model="newKeyForm.base_url" :placeholder="newKeyForm.provider === 'Custom' ? 'https://relay.example.com' : 'Optional provider override'" class="w-full bg-gray-800 border-gray-700 rounded-lg p-2.5 text-sm" /></div>
          <div><label class="text-xs font-bold text-gray-500 uppercase">Proxy URL</label><input v-model="newKeyForm.proxy_url" placeholder="Optional: http://127.0.0.1:7890" class="w-full bg-gray-800 border-gray-700 rounded-lg p-2.5 text-sm" /></div>
        </div>
        <div class="p-6 bg-gray-800/30 flex justify-end gap-3"><button @click="handleBulkAdd" class="bg-blue-600 px-6 py-2 rounded-lg font-bold">Import Keys</button></div>
      </div>
    </div>

    <div v-if="showAccountModal" class="fixed inset-0 bg-black/80 backdrop-blur-sm flex items-center justify-center p-4 z-50">
      <div class="bg-gray-900 border border-gray-800 rounded-2xl w-full max-w-md shadow-2xl">
        <div class="p-6 border-b border-gray-800 flex justify-between items-center"><h3 class="text-xl font-bold">New Account</h3><button @click="showAccountModal = false"><X class="w-6 h-6"/></button></div>
        <div class="p-6 space-y-4">
          <div><label class="text-xs font-bold text-gray-500 uppercase">Platform</label><input v-model="newAccountForm.platform" class="w-full bg-gray-800 border-gray-700 rounded-lg p-2.5 text-sm" /></div>
          <div><label class="text-xs font-bold text-gray-500 uppercase">Account</label><input v-model="newAccountForm.account" class="w-full bg-gray-800 border-gray-700 rounded-lg p-2.5 text-sm" /></div>
          <div><label class="text-xs font-bold text-gray-500 uppercase">Password</label><input v-model="newAccountForm.password" type="password" class="w-full bg-gray-800 border-gray-700 rounded-lg p-2.5 text-sm" /></div>
          <div><label class="text-xs font-bold text-gray-500 uppercase">URL</label><input v-model="newAccountForm.url" class="w-full bg-gray-800 border-gray-700 rounded-lg p-2.5 text-sm" /></div>
          <div><label class="text-xs font-bold text-gray-500 uppercase">TOTP Secret</label><input v-model="newAccountForm.totp_secret" placeholder="Optional raw base32 secret" class="w-full bg-gray-800 border-gray-700 rounded-lg p-2.5 text-sm" /></div>
          <div><label class="text-xs font-bold text-gray-500 uppercase">Favicon URL</label><input v-model="newAccountForm.favicon_url" placeholder="Optional override" class="w-full bg-gray-800 border-gray-700 rounded-lg p-2.5 text-sm" /></div>
        </div>
        <div class="p-6 bg-gray-800/30 flex justify-end gap-3"><button @click="handleAddAccount" class="bg-blue-600 px-6 py-2 rounded-lg font-bold">Save Account</button></div>
      </div>
    </div>
  </div>
</template>



