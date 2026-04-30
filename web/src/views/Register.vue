<script setup lang="ts">
import { ref } from 'vue';
import { useRouter } from 'vue-router';
import api from '../api';

const username = ref('');
const masterKey = ref('');
const inviteCode = ref('');
const router = useRouter();

const handleRegister = async () => {
  try {
    await api.post('/register', {
      username: username.value,
      master_key: masterKey.value,
      invite_code: inviteCode.value,
    });
    alert('注册成功，请登录！');
    router.push('/login');
  } catch (e: any) {
    alert('注册失败: ' + (e.response?.data?.error || '未知错误'));
  }
};
</script>

<template>
  <div class="vault-grid-bg relative flex min-h-screen items-center justify-center overflow-hidden p-4 text-slate-100">
    <div class="pointer-events-none absolute right-12 top-12 h-56 w-56 rounded-full bg-emerald-400/10 blur-3xl"></div>
    <div class="vault-surface relative w-full max-w-md overflow-hidden rounded-[2rem] p-8">
      <div class="absolute inset-x-8 top-0 h-px bg-gradient-to-r from-transparent via-emerald-300/60 to-transparent"></div>
      <div class="mb-8 text-center">
        <div class="mx-auto mb-5 flex h-14 w-14 items-center justify-center rounded-2xl border border-emerald-300/25 bg-emerald-300/10 shadow-lg shadow-emerald-500/10">
          <span class="text-2xl">◆</span>
        </div>
        <p class="mb-3 font-mono text-[11px] uppercase tracking-[0.32em] text-emerald-200/70">Invite Only Access</p>
        <h1 class="text-4xl font-semibold tracking-[-0.04em] text-white">Create Vault</h1>
        <p class="mt-3 text-sm leading-6 text-slate-400">Start with a local Master Key and an invitation-backed account boundary.</p>
      </div>

      <div class="space-y-4">
        <input v-model="username" placeholder="Username" class="vault-input w-full rounded-2xl px-4 py-3" autocomplete="username" />
        <input v-model="masterKey" type="password" placeholder="Master Key (Never Forget!)" class="vault-input w-full rounded-2xl px-4 py-3" autocomplete="new-password" />
        <input v-model="inviteCode" placeholder="Invite Code" class="vault-input w-full rounded-2xl px-4 py-3 font-mono" />
        <button @click="handleRegister" class="vault-primary-btn w-full rounded-2xl px-5 py-3.5 font-semibold">Create Account</button>
      </div>

      <p class="mt-6 text-center text-sm text-slate-500">
        Already have an account?
        <router-link to="/login" class="font-medium text-cyan-300 hover:text-cyan-200">Login</router-link>
      </p>
    </div>
  </div>
</template>
