/**
 * plugins/vuetify.ts
 *
 * Framework documentation: https://vuetifyjs.com`
 */

import type { IconProps, IconSet } from 'vuetify'
// Composables
import { h } from 'vue'
import { createVuetify } from 'vuetify'
import { VSvgIcon } from 'vuetify/components'
import { aliases } from 'vuetify/iconsets/mdi-svg'
import { iconPath } from './icons'
// Styles
import 'vuetify/styles'

// The default set draws `mdi-<name>` icons as inline SVG from the registry in
// ./icons (GLD-073); Vuetify's own aliases ($close, $expand...) are SVG paths.
const mdi: IconSet = {
  component: (props: IconProps) => {
    const name = String(props.icon)
    const path = iconPath(name)
    if (!path && import.meta.env.DEV) {
      console.warn(`Unregistered icon "${name}": add it to plugins/icons.ts`)
    }
    return h(VSvgIcon, { ...props, icon: path ?? '' })
  },
}

// https://vuetifyjs.com/en/introduction/why-vuetify/#feature-guides
export default createVuetify({
  icons: {
    defaultSet: 'mdi',
    aliases,
    sets: { mdi },
  },
  theme: {
    defaultTheme: 'system',
  },
})
