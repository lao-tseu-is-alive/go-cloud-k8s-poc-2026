<script setup lang="ts">
  import type { RelationshipGroup, SubjectKind } from '@/api/types'
  import type { RoleSection } from '@/utils/roleHeadings'
  import { computed, ref, watch } from 'vue'
  import { useI18n } from 'vue-i18n'
  import { summarizeRelationships } from '@/api/coreClient'
  import RoleGroup from '@/components/core/RoleGroup.vue'
  import { useApiErrors } from '@/composables/useApiErrors'
  import { roleHeading } from '@/utils/roleHeadings'
  import { kindIcon } from '@/utils/subjects'

  // The relationships of a subject organized for reading (GLD-063): one
  // section per kind of the other end (cases, documents, objects, actors...),
  // in each a group per role, titled from the subject on screen ("Client des
  // affaires suivantes") and sorted alphabetically, counted before it opens.
  // Only what the caller may read is counted. Emits the section totals for
  // the page's jump links.
  const props = defineProps<{ subjectId: string, reloadKey?: number }>()
  const emit = defineEmits<{ sections: [sections: RoleSection[]] }>()
  const { t, locale } = useI18n()
  const { report } = useApiErrors()

  const KIND_ORDER: SubjectKind[] = [
    'SUBJECT_KIND_CASE', 'SUBJECT_KIND_DOCUMENT', 'SUBJECT_KIND_THING', 'SUBJECT_KIND_ACTOR',
    'SUBJECT_KIND_ORG_UNIT', 'SUBJECT_KIND_USER', 'SUBJECT_KIND_GROUP',
  ]

  const groups = ref<RelationshipGroup[]>([])
  const loading = ref(false)

  const lang = computed(() => (String(locale.value).startsWith('fr') ? 'fr' : 'en'))

  const sections = computed<RoleSection[]>(() => {
    const byKind = new Map<SubjectKind, RoleSection>()
    for (const g of groups.value) {
      const kind = g.otherKind ?? 'SUBJECT_KIND_UNSPECIFIED'
      const type = g.relationshipType
      if (!type) continue
      let section = byKind.get(kind)
      if (!section) {
        section = { kind, anchor: `roles-${kind.replace('SUBJECT_KIND_', '').toLowerCase()}`, total: 0, groups: [] }
        byKind.set(kind, section)
      }
      section.total += g.count ?? 0
      section.groups.push({
        key: `${type.code}:${g.outgoing ? 'out' : 'in'}`,
        heading: roleHeading(type, g.outgoing ? 'SOURCE' : 'TARGET', g.count ?? 0, lang.value),
        group: g,
      })
    }
    const collator = new Intl.Collator(lang.value === 'fr' ? 'fr' : 'en', { sensitivity: 'base' })
    for (const section of byKind.values()) {
      section.groups = section.groups.toSorted((a, b) => collator.compare(a.heading, b.heading))
    }
    const rank = (k: SubjectKind) => {
      const i = KIND_ORDER.indexOf(k)
      return i === -1 ? KIND_ORDER.length : i
    }
    return [...byKind.values()].toSorted((a, b) => rank(a.kind) - rank(b.kind))
  })

  async function load () {
    loading.value = true
    try {
      groups.value = await summarizeRelationships(props.subjectId)
    } catch (error) {
      report(error)
    } finally {
      loading.value = false
    }
  }

  watch(sections, value => emit('sections', value))
  watch(() => [props.subjectId, props.reloadKey], load, { immediate: true })
</script>

<template>
  <div class="d-flex flex-column ga-4">
    <v-progress-linear v-if="loading && groups.length === 0" color="primary" indeterminate />
    <p v-else-if="sections.length === 0" class="text-medium-emphasis">{{ t('relGroups.empty') }}</p>

    <v-card
      v-for="section in sections"
      :id="section.anchor"
      :key="section.kind"
      :aria-labelledby="`${section.anchor}-title`"
      tag="section"
    >
      <div class="d-flex align-center ga-2 px-4 pt-4 pb-3">
        <v-icon color="primary" :icon="kindIcon(section.kind)" />
        <h2 :id="`${section.anchor}-title`" class="text-h6">{{ t(`relGroups.sections.${section.kind}`, t('relGroups.sections.other')) }}</h2>
        <span class="text-body-2 text-medium-emphasis">{{ t('relGroups.links', section.total) }} · {{ t('relGroups.roles', section.groups.length) }}</span>
      </div>

      <RoleGroup
        v-for="g in section.groups"
        :key="g.key"
        :group="g.group"
        :heading="g.heading"
        :subject-id="subjectId"
      />
    </v-card>
  </div>
</template>
