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
      invite_code: inviteCode.value 
    });
    alert('注册成功，请登录！');
    router.push('/login');
  } catch (e: any) {
    alert('注册失败: ' + (e.response?.data?.error || '未知错误'));
  }
};
</script>

<template>
  <div class="min-h-screen bg-gray-950 flex items-center justify-center p-4">
    <div class="bg-gray-900 p-8 rounded-2xl w-full max-w-md border border-gray-800">
      <h1 class="text-3xl font-bold text-white mb-6 text-center">Join AllInOne</h1>
      <div class="space-y-4">
        <input v-model="username" placeholder="Username" class="w-full bg-gray-800 p-3 rounded-lg text-white border border-gray-700" />
        <input v-model="masterKey" type="password" placeholder="Master Key (Never Forget!)" class="w-full bg-gray-800 p-3 rounded-lg text-white border border-gray-700" />
        <input v-model="inviteCode" placeholder="Invite Code" class="w-full bg-gray-800 p-3 rounded-lg text-white border border-gray-700" />
        <button @click="handleRegister" class="w-full bg-blue-600 py-3 rounded-lg font-bold text-white">Create Account</button>
      </div>
      <p class="mt-4 text-center text-sm text-gray-500">
        Already have an account? <router-link to="/login" class="text-blue-400">Login</router-link>
      </p>
    </div>
  </div>
</template>
