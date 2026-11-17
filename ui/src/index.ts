// The bundle's entry. The app imports this file and calls what it default
// exports, handing it the one object a module receives.
import { defineModule } from '@prusa/connect-ui/slots';

import BedMeshTab from './bed-mesh-tab.svelte';
import { holdHost } from './host';
import './styles.css';

export default defineModule((host) => {
  holdHost(host);

  // Held to `contributes` in module.yaml: the one tab on a printer's page.
  host.register.deviceTab('bed-mesh', BedMeshTab);
});
