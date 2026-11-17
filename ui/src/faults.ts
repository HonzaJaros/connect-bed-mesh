// This module's refusals in the reader's language. The core translates its
// own codes; a module's codes it leaves in English, so the codes faults.go
// registers are looked up in this module's catalogue first.
import { describeFault, type FaultText } from '@prusa/connect-ui/i18n';

import { heldHost } from './host';

const OWN = [
  'INVALID_BODY',
  'NO_MESH_IN_TEXT',
  'TEXT_TOO_LONG',
  'NOTE_TOO_LONG',
  'PRINTER_NOT_FOUND',
  'MESH_NOT_FOUND',
  'UNAVAILABLE',
] as const;

export const faultText = (thrown: unknown): FaultText => {
  const described = describeFault(thrown);
  const own = OWN.find((code) => described.code?.endsWith(code));

  if (own === undefined) return described;

  return {
    ...described,
    title: heldHost().t(`bedmesh.fault.${own}`),
  };
};
