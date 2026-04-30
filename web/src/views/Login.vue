<script setup lang="ts">
import { ref } from 'vue';
import { useRouter } from 'vue-router';
import api, { getApiErrorMessage } from '../api';
import { useAuthStore } from '../store/auth';

const username = ref('');
const masterKey = ref('');
const router = useRouter();
const auth = useAuthStore();

const handleLogin = async () => {
  try {
    const res = await api.post('/login', { username: username.value, master_key: masterKey.value });
    auth.setToken(res.data.token, res.data.role);
    router.push('/');
  } catch (error) {
    alert('登录失败：' + getApiErrorMessage(error));
  }
};
</script>

<template>
  <div class="vault-grid-bg relative flex min-h-screen items-center justify-center overflow-hidden p-4 text-slate-100">
    <div class="pointer-events-none absolute left-1/2 top-16 h-48 w-48 -translate-x-1/2 rounded-full bg-cyan-400/10 blur-3xl"></div>
    <div class="vault-surface relative w-full max-w-md overflow-hidden rounded-[2rem] p-8">
      <div class="absolute inset-x-8 top-0 h-px bg-gradient-to-r from-transparent via-cyan-300/60 to-transparent"></div>
      <div class="mb-8 text-center">
        <div class="mx-auto mb-5 flex h-14 w-14 items-center justify-center rounded-2xl border border-cyan-300/25 bg-cyan-300/10 shadow-lg shadow-cyan-500/10">
          <span class="text-2xl">◇</span>
        </div>
        <p class="mb-3 font-mono text-[11px] uppercase tracking-[0.32em] text-cyan-200/70">Zero-Knowledge Vault</p>
        <h1 class="text-4xl font-semibold tracking-[-0.04em] text-white">All In One Key</h1>
        <p class="mt-3 text-sm leading-6 text-slate-400">Unlock encrypted API keys, accounts, and audit trails with your local Master Key.</p>
      </div>

      <div class="space-y-4">
        <div>
          <label class="mb-2 block text-xs font-medium uppercase tracking-[0.18em] text-slate-500">Username</label>
          <input v-model="username" type="text" class="vault-input w-full rounded-2xl px-4 py-3" autocomplete="username" />
        </div>
        <div>
          <label class="mb-2 block text-xs font-medium uppercase tracking-[0.18em] text-slate-500">Master Key</label>
          <input v-model="masterKey" type="password" class="vault-input w-full rounded-2xl px-4 py-3" autocomplete="current-password" />
        </div>
        <button @click="handleLogin" class="vault-primary-btn mt-2 w-full rounded-2xl px-5 py-3.5 font-semibold">
          Unlock Vault
        </button>
      </div>

      <div class="mt-7 rounded-2xl border border-emerald-300/10 bg-emerald-300/[0.04] p-4 text-center text-xs leading-5 text-slate-400">
        Master Key never touches persistent storage. Session tokens stay opaque.
      </div>

      <p class="mt-6 text-center text-sm text-slate-500">
        New here?
        <router-link to="/register" class="font-medium text-cyan-300 hover:text-cyan-200">Create an account</router-link>
      </p>
    </div>
  </div>
</template>
