<script setup lang="ts">
  import type { ActorFormModel } from '@/components/actor/actorForm'
  import { computed, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import ActorAddressesEditor from '@/components/actor/ActorAddressesEditor.vue'
  import ActorContactsEditor from '@/components/actor/ActorContactsEditor.vue'
  import { personDisplayName, SALUTATIONS } from '@/components/actor/actorForm'
  import ActorKindSelect from '@/components/actor/ActorKindSelect.vue'
  import OrganizationCategorySelect from '@/components/actor/OrganizationCategorySelect.vue'
  import { useI18nEnum } from '@/composables/useI18nEnum'
  import { maxLength, required } from '@/utils/validation'

  // Shared actor form fields, used by both the create page and the detail edit
  // panel. The kind is immutable once created (a PERSON never becomes an ORG), so
  // `lockKind` disables the kind selector in edit mode.
  const props = defineProps<{ lockKind?: boolean }>()
  const model = defineModel<ActorFormModel>({ required: true })

  const { t } = useI18n()

  const { enumLabel } = useI18nEnum()
  const isOrganization = computed(() => model.value.actorKind === 'ACTOR_KIND_ORGANIZATION')
  const salutationItems = computed(() => SALUTATIONS.map(s => ({ value: s, title: enumLabel('Salutation', s) })))

  // A person's usual name follows "<first> <last>" until the user types another one.
  watch(
    () => [model.value.firstName, model.value.lastName] as const,
    ([first, last], [oldFirst, oldLast]) => {
      if (isOrganization.value) return
      const previous = personDisplayName(oldFirst ?? '', oldLast ?? '')
      if (model.value.displayName.trim() === '' || model.value.displayName === previous) {
        model.value.displayName = personDisplayName(first, last)
      }
    },
  )
</script>

<template>
  <div>
    <ActorKindSelect v-model="model.actorKind" :disabled="props.lockKind" :rules="[required(t)]" />

    <!-- PERSON minimal identity: enough to identify and address the person -->
    <v-row v-if="!isOrganization" dense>
      <v-col cols="12" sm="3">
        <v-select
          v-model="model.salutation"
          item-title="title"
          item-value="value"
          :items="salutationItems"
          :label="t('fields.actor.salutation')"
        />
      </v-col>

      <v-col cols="12" sm="4">
        <v-text-field v-model="model.firstName" :label="t('fields.actor.first_name')" :rules="[maxLength(t, 100)]" />
      </v-col>

      <v-col cols="12" sm="5">
        <v-text-field v-model="model.lastName" :label="t('fields.actor.last_name')" :rules="[required(t), maxLength(t, 100)]" />
      </v-col>
    </v-row>

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

    <!-- PERSON register link -->
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

    <ActorAddressesEditor v-model="model.addresses" :actor-kind="model.actorKind" />

    <v-divider class="my-4" />

    <ActorContactsEditor v-model="model.contacts" />
  </div>
</template>
