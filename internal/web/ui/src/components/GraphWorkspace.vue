<template>
  <ElContainer class="graph-workspace">
    <ElMain class="graph-workspace__canvas"><slot name="canvas" /></ElMain>
    <ElAside class="graph-workspace__inspector" width="328px"><slot name="inspector" /></ElAside>
  </ElContainer>
</template>

<style scoped>
.graph-workspace {
  min-width: 0;
  min-height: 0;
  flex: 1;
  overflow: hidden;
  border: 1px solid var(--border-subtle);
  border-radius: var(--locus-radius-md);
  background: var(--surface-panel);
}

.graph-workspace__canvas {
  min-width: 0;
  padding: 0;
  overflow: hidden;
}

.graph-workspace__inspector {
  min-width: 0;
  overflow: hidden;
}

@media (max-width: 1100px) {
  .graph-workspace {
    min-height: 0;
    display: block;
  }

  .graph-workspace__canvas {
    height: 540px;
  }

  .graph-workspace__inspector {
    width: 100% !important;
    border-top: 1px solid var(--border-subtle);
  }
}

@media (max-width: 600px) {
  .graph-workspace__canvas {
    height: 440px;
  }
}
</style>
