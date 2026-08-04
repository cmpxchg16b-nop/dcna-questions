// Type declarations for *.xml imports, loaded through xml-loader (see
// turbopack.rules in next.config.ts). xml-loader parses with xml2js
// defaults: element attributes live under `$`, and repeated child elements
// are arrays keyed by tag name. loginOptions.xml is the only XML import
// today, so the wildcard module is typed to its shape.
declare module "*.xml" {
  interface LoginOptionXmlEntry {
    $: {
      kind: string;
      name: string;
      displayName: string;
      loginURL: string;
      label?: string;
    };
  }
  const content: {
    loginOptions: {
      loginOption?: LoginOptionXmlEntry[];
    };
  };
  export default content;
}
