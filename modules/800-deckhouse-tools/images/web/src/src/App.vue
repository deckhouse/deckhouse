<script lang="ts">
interface ToolListItem {
  name: string,
  version: string;
  links: Array<{ link: string, platform: "Linux (Intel)" | "MacOS (Intel)" | "MacOS (ARM)" | "Windows (Intel)" }>
}

export default {
  data() {
    return {
      tools: [] as ToolListItem[],
      isLoading: false,
    }
  },
  mounted() { this.getToolsList() },
  methods: {
    async getToolsList() {
      this.isLoading = true
      const url = "/tools.json";
      try {
        const response = await fetch(url);
        if (!response.ok) {
          alert(`Response status: ${response.status}. See console for details.`);
          console.error(response)
          this.isLoading = false
          return
        }

        this.tools = await response.json()
        this.isLoading = false
      } catch (error) {
        console.error(error);
        this.isLoading = false
      }
    }
  }
}
</script>

<template>
  <main>
    <div class="card-spacer"></div>
    <div class="card">
      <div class="card-content">
        <img src="/deckhouse-logo-icon.svg" class="deckhouse-logo">
        <img src="/deckhouse-logo-title.svg" class="deckhouse-title">
        <h2 class="tools-title">Tools</h2>

        <div v-if="isLoading">
          <div style="text-align: center">Loading...</div>
        </div>

        <div v-if="!isLoading && tools.length === 0">
          <h3 class="tools-subtitle">No tools available</h3>
        </div>

        <div v-else-if="!isLoading" v-for="tool in tools" style="padding-top: .5rem;">
          <h3 class="tools-subtitle">{{ tool.name }}</h3>
          <ul class="links">
            <li v-for="link in tool.links">
              <a :href="link.link" class="download-link">
                {{ tool.name }} {{ tool.version }} for {{ link.platform }}
              </a>
            </li>
          </ul>
        </div>
      </div>
    </div>
  </main>
</template>

<style scoped>
.card {
  position: relative;
  margin: 0 auto;
  width: 500px;
  border-radius: 0.375rem;
  background-color: white;
  padding: 70px 60px;
}

.card-spacer {
  height: 100px;
  margin-bottom: auto;
}

.card-content {
  flex-direction: column;
}

.deckhouse-logo {
  position: absolute;
  left: 50%;
  top: 0;
  transform: translate(-50%, -50%);
}

.deckhouse-title {
  margin-left: auto;
  margin-right: auto;
  margin-bottom: 2rem;
  display: block;
}

.tools-title, .tools-subtitle {
  text-align: center;
}

.links {
  list-style: none;
  padding: 0;
}

.links li {
  margin: 1.5rem 1rem;
  padding: 1rem;
  border: 1px solid rgba(0,0,0, 0.2);
  border-radius: .375rem;
}

.download-link {
  text-decoration: none;
  display: block;
  font-weight: 600;
  color: rgb(30, 41, 59);
}
</style>
