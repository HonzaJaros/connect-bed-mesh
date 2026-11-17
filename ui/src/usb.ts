// Reading the mesh straight from the printer's USB port, in a browser that
// has the Web Serial API (Chrome, Edge, Opera on a desktop) and a page served
// over HTTPS or from localhost.
//
// The person picks the printer's port in the browser's own dialog. The page
// sends M420 V, collects what the printer prints until the report is over,
// and closes the port again. Nothing else is sent.

// The slice of the Web Serial API this file uses. TypeScript's DOM library
// does not carry it yet.
interface SerialPortLike {
  open(options: { baudRate: number }): Promise<void>;
  close(): Promise<void>;
  readonly readable: ReadableStream<Uint8Array> | null;
  readonly writable: WritableStream<Uint8Array> | null;
}

interface SerialLike {
  requestPort(): Promise<SerialPortLike>;
}

const serial = (): SerialLike | undefined =>
  (globalThis.navigator as Navigator & { serial?: SerialLike }).serial;

/** Whether this browser, on this page, can open a USB serial port. */
export const usbAvailable = (): boolean =>
  serial() !== undefined && globalThis.isSecureContext;

/** Buddy printers talk at this rate on their USB port. */
const BAUD_RATE = 115200;
/** How long to wait for the report before giving up, in milliseconds. */
const TIMEOUT = 15000;

/**
 * Asks the person for the printer's port, sends `command` and returns every
 * line the printer printed until it reported leveling on or off, which
 * M420 V prints after the map, or until the timeout.
 */
export const readOverUsb = async (
  command = 'M420 V',
  onText?: (text: string) => void,
): Promise<string> => {
  const api = serial();
  if (!api) throw new Error('usb-unavailable');

  const port = await api.requestPort();
  await port.open({ baudRate: BAUD_RATE });

  let text = '';
  const reader = port.readable?.getReader();
  const writer = port.writable?.getWriter();

  try {
    if (!reader || !writer) throw new Error('usb-unavailable');

    await writer.write(new TextEncoder().encode(`${command}\n`));

    const decoder = new TextDecoder();
    const deadline = Date.now() + TIMEOUT;
    let timer: ReturnType<typeof setTimeout> | undefined;

    while (Date.now() < deadline) {
      const timeout = new Promise<{ done: true; value: undefined }>(
        (resolve) => {
          timer = setTimeout(() => {
            resolve({ done: true, value: undefined });
          }, deadline - Date.now());
        },
      );
      const { done, value } = await Promise.race([reader.read(), timeout]);
      clearTimeout(timer);
      if (done) break;

      text += decoder.decode(value, { stream: true });
      onText?.(text);

      if (/Bed Leveling (ON|OFF)|Invalid mesh/.test(text)) break;
    }

    return text;
  } finally {
    await reader?.cancel().catch(() => undefined);
    reader?.releaseLock();
    writer?.releaseLock();
    await port.close().catch(() => undefined);
  }
};
