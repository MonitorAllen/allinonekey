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
  <div class="min-h-screen bg-gray-900 flex items-center justify-center p-4">
    <div class="bg-gray-800 p-8 rounded-2xl shadow-2xl w-full max-w-md border border-gray-700">
      <h1 class="text-3xl font-bold text-white mb-2 text-center">All In One Key</h1>
      <p class="text-gray-400 text-center mb-8 italic">Zero-Knowledge Vault</p>
      
      <div class="space-y-4">
        <div>
          <label class="block text-sm font-medium text-gray-300 mb-1">Username</label>
          <input v-model="username" type="text" class="w-full bg-gray-700 border-gray-600 rounded-lg text-white focus:ring-blue-500 focus:border-blue-500 p-2.5" />
        </div>
        <div>
          <label class="block text-sm font-medium text-gray-300 mb-1">Master Key</label>
          <input v-model="masterKey" type="password" class="w-full bg-gray-700 border-gray-600 rounded-lg text-white focus:ring-blue-500 focus:border-blue-500 p-2.5" />
        </div>
        <button @click="handleLogin" class="w-full bg-blue-600 hover:bg-blue-700 text-white font-bold py-3 rounded-lg transition-colors">
          Unlock Vault
        </button>
      </div>
      
      <p class="mt-6 text-center text-sm text-gray-500">
        New here? <router-link to="/register" class="text-blue-400 hover:text-blue-300">Create an account</router-link>
      </p>
      <p class="mt-3 text-center text-xs text-gray-500">
        Your Master Key never touches our persistent storage.
      </p>
    </div>
  </div>
</template>

