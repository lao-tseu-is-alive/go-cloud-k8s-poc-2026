import type { SubjectRelationship } from '@/api/types'
/**
 * Link / unlink handlers shared by the detail pages (case, document, actor):
 * dialog state, the API calls, feedback and the reload of the page. Ending a
 * relationship is handled by the relationship table's own dialog.
 */
import type { Ref } from 'vue'
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { linkSubjects, unlinkSubjects } from '@/api/coreClient'
import { useApiErrors } from '@/composables/useApiErrors'
import { useUiStore } from '@/stores/ui'

/** The payload emitted by LinkSubjectDialog. */
export interface LinkPayload {
  targetSubjectId: string
  relationshipTypeCode: string
  roleDetail: string
}

type LinkCall = (sourceId: string, targetId: string, typeCode: string, roleDetail?: string) => Promise<unknown>

export function useSubjectLinks (sourceId: Ref<string>, reload: () => Promise<void>, link: LinkCall = linkSubjects) {
  const { t } = useI18n()
  const { report } = useApiErrors()
  const ui = useUiStore()
  const linkOpen = ref(false)
  const linkBusy = ref(false)

  async function doLink (payload: LinkPayload): Promise<void> {
    linkBusy.value = true
    try {
      await link(sourceId.value, payload.targetSubjectId, payload.relationshipTypeCode, payload.roleDetail || undefined)
      ui.notify(t('messages.relationship.linked'), 'success')
      linkOpen.value = false
      await reload()
    } catch (error) {
      report(error)
    } finally {
      linkBusy.value = false
    }
  }

  async function doUnlink (rel: SubjectRelationship): Promise<void> {
    try {
      await unlinkSubjects(rel.id, '')
      await reload()
    } catch (error) {
      report(error)
    }
  }

  return { linkOpen, linkBusy, doLink, doUnlink }
}
