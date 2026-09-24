<script setup lang="ts">
  import type { ActorFormModel } from '@/components/actor/actorForm'
  import { computed } from 'vue'
  import { useI18n } from 'vue-i18n'
  import ActorContactsEditor from '@/components/actor/ActorContactsEditor.vue'
  import ActorKindSelect from '@/components/actor/ActorKindSelect.vue'
  import OrganizationCategorySelect from '@/components/actor/OrganizationCategorySelect.vue'
  import { maxLength, required } from '@/utils/validation'

  // Shared actor form fields, used by both the create page and the detail edit
  // panel. The kind is immutable once created (a PERSON never becomes an ORG), so
  // `lockKind` disables the kind selector in edit mode.
  const props = defineProps<{ lockKind?: boolean }>()
  const model = defineModel<ActorFormModel>({ required: true })

  const { t } = useI18n()

  const isOrganization = computed(() => model.value.actorKind === 'ACTOR_KIND_ORGANIZATION')
</script>

<template>
  <div>
    <ActorKindSelect v-model="model.actorKind" :disabled="props.lockKind" :rules="[required(t)]" />

    <v-text-field
      v-model="model.displayName"
      :hint="t(isOrganization ? 'fields.actor.display_name_hint_org' : 'fields.actor.display_name_hint')"
      :label="t('fields.actor.display_name')"
      persistent-hint
      :rules="[required(t), maxLength(t, 200)]"
    />

    <!-- ORGANIZATION specialization -->
    <template v-if="isOrganization">
      <v-text-field
        v-model="model.legalName"
        class="mt-2"
        :hint="t('fields.actor.legal_name_hint')"
        :label="t('fields.actor.legal_name')"
        persistent-hint
        :rules="[required(t), maxLength(t, 200)]"
      />

      <OrganizationCategorySelect v-model="model.categoryCode" class="mt-2" clearable />

      <v-textarea
        v-model="model.orgComplement"
        auto-grow
        :hint="t('fields.actor.org_complement_hint')"
        :label="t('fields.actor.org_complement')"
        persistent-hint
        rows="1"
        :rules="[maxLength(t, 1000)]"
      />
    </template>

    <!-- PERSON specialization (no personal data: register link only) -->
    <template v-else>
      <v-checkbox v-model="model.isChRegister" :label="t('fields.actor.is_ch_register')" />

      <v-text-field
        v-if="model.isChRegister"
        v-model="model.chRegisterRef"
        :hint="t('fields.actor.ch_register_ref_hint')"
        :label="t('fields.actor.ch_register_ref')"
        persistent-hint
      />
    </template>

    <v-divider class="my-4" />

    <ActorContactsEditor v-model="model.contacts" />
  </div>
</template>
