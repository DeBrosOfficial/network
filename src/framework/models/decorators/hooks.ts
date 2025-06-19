export function BeforeCreate(target: any, propertyKey: string, descriptor: PropertyDescriptor) {
  registerHook(target, 'beforeCreate', descriptor.value);
}

export function AfterCreate(target: any, propertyKey: string, descriptor: PropertyDescriptor) {
  registerHook(target, 'afterCreate', descriptor.value);
}

export function BeforeUpdate(target: any, propertyKey: string, descriptor: PropertyDescriptor) {
  registerHook(target, 'beforeUpdate', descriptor.value);
}

export function AfterUpdate(target: any, propertyKey: string, descriptor: PropertyDescriptor) {
  registerHook(target, 'afterUpdate', descriptor.value);
}

export function BeforeDelete(target: any, propertyKey: string, descriptor: PropertyDescriptor) {
  registerHook(target, 'beforeDelete', descriptor.value);
}

export function AfterDelete(target: any, propertyKey: string, descriptor: PropertyDescriptor) {
  registerHook(target, 'afterDelete', descriptor.value);
}

export function BeforeSave(target: any, propertyKey: string, descriptor: PropertyDescriptor) {
  registerHook(target, 'beforeSave', descriptor.value);
}

export function AfterSave(target: any, propertyKey: string, descriptor: PropertyDescriptor) {
  registerHook(target, 'afterSave', descriptor.value);
}

function registerHook(target: any, hookName: string, hookFunction: Function): void {
  // Initialize hooks map if it doesn't exist
  if (!target.constructor.hooks) {
    target.constructor.hooks = new Map();
  }

  // Get existing hooks for this hook name
  const existingHooks = target.constructor.hooks.get(hookName) || [];

  // Add the new hook
  existingHooks.push(hookFunction);

  // Store updated hooks array
  target.constructor.hooks.set(hookName, existingHooks);

  console.log(`Registered ${hookName} hook for ${target.constructor.name}`);
}

// Utility function to get hooks for a specific event
export function getHooks(target: any, hookName: string): Function[] {
  if (!target.constructor.hooks) {
    return [];
  }
  return target.constructor.hooks.get(hookName) || [];
}

// Export decorator types for TypeScript
export type HookDecorator = (
  target: any,
  propertyKey: string,
  descriptor: PropertyDescriptor,
) => void;
