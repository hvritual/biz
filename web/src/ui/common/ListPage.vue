<template>
  <section class="flex min-w-0 flex-col gap-4" data-ui-pattern="ListPage">
    <header v-if="$slots.heading" data-ui-region="heading"><slot name="heading" /></header>
    <div v-if="$slots.metrics" data-ui-region="metrics"><slot name="metrics" /></div>
    <div v-if="$slots.filters" data-ui-region="filters"><slot name="filters" /></div>
    <section class="card data-panel" data-ui-region="data-panel">
      <div v-if="$slots.toolbar" class="table-toolbar" data-ui-region="toolbar"><slot name="toolbar" /></div>
      <slot />
      <footer v-if="$slots.pagination" data-ui-region="pagination"><slot name="pagination" /></footer>
    </section>
  </section>
</template>
