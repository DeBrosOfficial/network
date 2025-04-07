// custom.d.ts
declare module '@orbitdb/core' {
  // Import the types from @constl/orbit-db-types
  import { OrbitDBTypes } from '@orbitdb/core-types';

  // Assuming @orbitdb/core exports an interface or type you want to override
  // Replace 'OrbitDB' with the actual export name from @orbitdb/core you want to type
  export interface OrbitDB extends OrbitDBTypes {
    // You can add additional properties or methods here if needed
  }

  // If @orbitdb/core exports a default export, you might need something like:
  // export default interface OrbitDBDefault extends OrbitDBTypes {}
}
