<script setup>
import { ref } from 'vue'

const mobileMenuOpen = ref(false)
</script>

<template>
  <div class="min-h-screen bg-gray-100">
    <nav class="bg-white shadow-sm">
      <div class="max-w-7xl mx-auto px-4">
        <div class="flex justify-between h-16">
          <div class="flex items-center">
            <router-link to="/" class="flex items-center">
              <span class="text-xl font-bold text-gray-800">TCG Tracker</span>
            </router-link>
          </div>

          <!-- Desktop navigation -->
          <div class="hidden md:flex items-center space-x-8">
            <router-link
              to="/"
              class="text-gray-600 hover:text-gray-900 px-3 py-2 rounded-md"
              active-class="text-blue-600 font-medium"
            >
              Dashboard
            </router-link>
            <router-link
              to="/collection"
              class="text-gray-600 hover:text-gray-900 px-3 py-2 rounded-md"
              active-class="text-blue-600 font-medium"
            >
              Collection
            </router-link>
            <router-link
              to="/add"
              class="bg-blue-600 text-white px-4 py-2 rounded-lg hover:bg-blue-700 transition"
            >
              + Add Card
            </router-link>
          </div>

          <!-- Mobile menu button -->
          <div class="md:hidden flex items-center">
            <button
              @click="mobileMenuOpen = !mobileMenuOpen"
              class="text-gray-600 hover:text-gray-900"
            >
              <svg class="h-6 w-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path
                  v-if="!mobileMenuOpen"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M4 6h16M4 12h16M4 18h16"
                />
                <path
                  v-else
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M6 18L18 6M6 6l12 12"
                />
              </svg>
            </button>
          </div>
        </div>
      </div>

      <!-- Mobile menu -->
      <div v-if="mobileMenuOpen" class="md:hidden border-t">
        <div class="px-2 pt-2 pb-3 space-y-1">
          <router-link
            to="/"
            class="block px-3 py-2 rounded-md text-gray-600 hover:text-gray-900 hover:bg-gray-50"
            @click="mobileMenuOpen = false"
          >
            Dashboard
          </router-link>
          <router-link
            to="/collection"
            class="block px-3 py-2 rounded-md text-gray-600 hover:text-gray-900 hover:bg-gray-50"
            @click="mobileMenuOpen = false"
          >
            Collection
          </router-link>
          <router-link
            to="/add"
            class="block px-3 py-2 rounded-md bg-blue-600 text-white"
            @click="mobileMenuOpen = false"
          >
            + Add Card
          </router-link>
        </div>
      </div>
    </nav>

    <main class="max-w-7xl mx-auto px-4 py-8">
      <router-view />
    </main>
  </div>
</template>
