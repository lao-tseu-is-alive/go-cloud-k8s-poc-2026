import vuetify from 'eslint-config-vuetify'
import sonarjs from 'eslint-plugin-sonarjs'
import pluginVue from 'eslint-plugin-vue'
import vueA11y from 'eslint-plugin-vuejs-accessibility'

// Sonar parity: the rules below mirror SonarQube findings fixed in this
// repository so they fail `bun run lint` (and `make check`) locally instead of
// resurfacing on the SonarCloud dashboard. See AGENTS.md "Code quality rules".
export default vuetify(
  {
    ts: true,
  },
  {
    name: 'goeland/sonar-parity',
    plugins: {
      // Same module instance as the one registered by eslint-config-vuetify.
      'vue': pluginVue,
      sonarjs,
      'vuejs-accessibility': vueA11y,
    },
    rules: {
      // typescript:S3776 — functions stay at or below the Sonar threshold.
      'sonarjs/cognitive-complexity': ['error', 15],
      // Web:MouseEventWithoutKeyboardEquivalentCheck — clickable elements need a keyboard path.
      'vuejs-accessibility/click-events-have-key-events': 'error',
      'vuejs-accessibility/mouse-events-have-key-events': 'error',
      // The plugin exempts <tr>; clickable table rows (our list pages) must be
      // reachable from the keyboard too.
      'vue/no-restricted-syntax': ['error', {
        selector: 'VElement[name="tr"] > VStartTag:has(VDirectiveKey[argument.name="click"]):not(:has(VDirectiveKey[argument.name="keydown"]))',
        message: 'A clickable <tr> needs a keyboard equivalent (@keydown.enter, tabindex="0").',
      }],
    },
  },
)
