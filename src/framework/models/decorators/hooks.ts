export function BeforeCreate(
  target?: any,
  propertyKey?: string,
  descriptor?: PropertyDescriptor,
): any {
  // If used as @BeforeCreate (without parentheses)
  if (target && propertyKey && descriptor) {
    registerHook(target, 'beforeCreate', descriptor.value);
    return descriptor;
  }

  // If used as @BeforeCreate() (with parentheses)
  return function (target: any, propertyKey: string, descriptor: PropertyDescriptor) {
    // Handle case where descriptor might be undefined
    if (!descriptor) {
      // For method decorators, we need to get the method from the target
      const method = target[propertyKey];
      if (typeof method === 'function') {
        registerHook(target, 'beforeCreate', method);
      }
      return;
    }
    registerHook(target, 'beforeCreate', descriptor.value);
    return descriptor;
  };
}

export function AfterCreate(
  target?: any,
  propertyKey?: string,
  descriptor?: PropertyDescriptor,
): any {
  if (target && propertyKey && descriptor) {
    registerHook(target, 'afterCreate', descriptor.value);
    return descriptor;
  }

  return function (target: any, propertyKey: string, descriptor: PropertyDescriptor) {
    // Handle case where descriptor might be undefined
    if (!descriptor) {
      // For method decorators, we need to get the method from the target
      const method = target[propertyKey];
      if (typeof method === 'function') {
        registerHook(target, 'afterCreate', method);
      }
      return;
    }
    registerHook(target, 'afterCreate', descriptor.value);
    return descriptor;
  };
}

export function BeforeUpdate(
  target?: any,
  propertyKey?: string,
  descriptor?: PropertyDescriptor,
): any {
  if (target && propertyKey && descriptor) {
    registerHook(target, 'beforeUpdate', descriptor.value);
    return descriptor;
  }

  return function (target: any, propertyKey: string, descriptor: PropertyDescriptor) {
    // Handle case where descriptor might be undefined
    if (!descriptor) {
      // For method decorators, we need to get the method from the target
      const method = target[propertyKey];
      if (typeof method === 'function') {
        registerHook(target, 'beforeUpdate', method);
      }
      return;
    }
    registerHook(target, 'beforeUpdate', descriptor.value);
    return descriptor;
  };
}

export function AfterUpdate(
  target?: any,
  propertyKey?: string,
  descriptor?: PropertyDescriptor,
): any {
  if (target && propertyKey && descriptor) {
    registerHook(target, 'afterUpdate', descriptor.value);
    return descriptor;
  }

  return function (target: any, propertyKey: string, descriptor: PropertyDescriptor) {
    // Handle case where descriptor might be undefined
    if (!descriptor) {
      // For method decorators, we need to get the method from the target
      const method = target[propertyKey];
      if (typeof method === 'function') {
        registerHook(target, 'afterUpdate', method);
      }
      return;
    }
    registerHook(target, 'afterUpdate', descriptor.value);
    return descriptor;
  };
}

export function BeforeDelete(
  target?: any,
  propertyKey?: string,
  descriptor?: PropertyDescriptor,
): any {
  if (target && propertyKey && descriptor) {
    registerHook(target, 'beforeDelete', descriptor.value);
    return descriptor;
  }

  return function (target: any, propertyKey: string, descriptor: PropertyDescriptor) {
    // Handle case where descriptor might be undefined
    if (!descriptor) {
      // For method decorators, we need to get the method from the target
      const method = target[propertyKey];
      if (typeof method === 'function') {
        registerHook(target, 'beforeDelete', method);
      }
      return;
    }
    registerHook(target, 'beforeDelete', descriptor.value);
    return descriptor;
  };
}

export function AfterDelete(
  target?: any,
  propertyKey?: string,
  descriptor?: PropertyDescriptor,
): any {
  if (target && propertyKey && descriptor) {
    registerHook(target, 'afterDelete', descriptor.value);
    return descriptor;
  }

  return function (target: any, propertyKey: string, descriptor: PropertyDescriptor) {
    // Handle case where descriptor might be undefined
    if (!descriptor) {
      // For method decorators, we need to get the method from the target
      const method = target[propertyKey];
      if (typeof method === 'function') {
        registerHook(target, 'afterDelete', method);
      }
      return;
    }
    registerHook(target, 'afterDelete', descriptor.value);
    return descriptor;
  };
}

export function BeforeSave(
  target?: any,
  propertyKey?: string,
  descriptor?: PropertyDescriptor,
): any {
  if (target && propertyKey && descriptor) {
    registerHook(target, 'beforeSave', descriptor.value);
    return descriptor;
  }

  return function (target: any, propertyKey: string, descriptor: PropertyDescriptor) {
    // Handle case where descriptor might be undefined
    if (!descriptor) {
      // For method decorators, we need to get the method from the target
      const method = target[propertyKey];
      if (typeof method === 'function') {
        registerHook(target, 'beforeSave', method);
      }
      return;
    }
    registerHook(target, 'beforeSave', descriptor.value);
    return descriptor;
  };
}

export function AfterSave(
  target?: any,
  propertyKey?: string,
  descriptor?: PropertyDescriptor,
): any {
  if (target && propertyKey && descriptor) {
    registerHook(target, 'afterSave', descriptor.value);
    return descriptor;
  }

  return function (target: any, propertyKey: string, descriptor: PropertyDescriptor) {
    // Handle case where descriptor might be undefined
    if (!descriptor) {
      // For method decorators, we need to get the method from the target
      const method = target[propertyKey];
      if (typeof method === 'function') {
        registerHook(target, 'afterSave', method);
      }
      return;
    }
    registerHook(target, 'afterSave', descriptor.value);
    return descriptor;
  };
}

function registerHook(target: any, hookName: string, hookFunction: Function): void {
  // Initialize hooks map if it doesn't exist, inheriting from parent
  if (!target.constructor.hasOwnProperty('hooks')) {
    // Copy hooks from parent class if they exist
    const parentHooks = target.constructor.hooks || new Map();
    target.constructor.hooks = new Map();

    // Copy all parent hooks
    for (const [name, hooks] of parentHooks.entries()) {
      target.constructor.hooks.set(name, [...hooks]);
    }
  }

  // Get existing hooks for this hook name
  const existingHooks = target.constructor.hooks.get(hookName) || [];

  // Add the new hook (store the function name for the tests)
  const functionName = hookFunction.name || 'anonymous';
  existingHooks.push(functionName);

  // Store updated hooks array
  target.constructor.hooks.set(hookName, existingHooks);

  console.log(`Registered ${hookName} hook for ${target.constructor.name}`);
}

// Utility function to get hooks for a specific event or all hooks
export function getHooks(target: any, hookName?: string): string[] | Record<string, string[]> {
  // Handle both class constructors and instances
  let current = target;
  if (target.constructor && target.constructor !== Function) {
    current = target.constructor;
  }

  // Collect hooks from the entire prototype chain
  const allHooks: Record<string, string[]> = {};

  while (current && current !== Function && current !== Object) {
    if (current.hooks) {
      for (const [name, hookFunctions] of current.hooks.entries()) {
        if (!allHooks[name]) {
          allHooks[name] = [];
        }
        // Add hooks from this level (parent hooks first, child hooks last)
        allHooks[name] = [...hookFunctions, ...allHooks[name]];
      }
    }
    current = Object.getPrototypeOf(current);
    // Stop if we've reached the base class or gone too far
    if (current === Function.prototype || current === Object.prototype) {
      break;
    }
  }

  if (hookName) {
    return allHooks[hookName] || [];
  } else {
    return allHooks;
  }
}

// Export decorator types for TypeScript
export type HookDecorator = (
  target: any,
  propertyKey: string,
  descriptor: PropertyDescriptor,
) => void;
