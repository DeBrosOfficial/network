export function BeforeCreate(target?: any, propertyKey?: string, descriptor?: PropertyDescriptor): any {
  if (target && propertyKey && descriptor) {
    // Used as @BeforeCreate (without parentheses)
    registerHook(target, 'beforeCreate', descriptor.value);
  } else {
    // Used as @BeforeCreate() (with parentheses)
    return function(target: any, propertyKey: string, descriptor: PropertyDescriptor) {
      registerHook(target, 'beforeCreate', descriptor.value);
    };
  }
}

export function AfterCreate(target?: any, propertyKey?: string, descriptor?: PropertyDescriptor): any {
  if (target && propertyKey && descriptor) {
    registerHook(target, 'afterCreate', descriptor.value);
  } else {
    return function(target: any, propertyKey: string, descriptor: PropertyDescriptor) {
      registerHook(target, 'afterCreate', descriptor.value);
    };
  }
}

export function BeforeUpdate(target?: any, propertyKey?: string, descriptor?: PropertyDescriptor): any {
  if (target && propertyKey && descriptor) {
    registerHook(target, 'beforeUpdate', descriptor.value);
  } else {
    return function(target: any, propertyKey: string, descriptor: PropertyDescriptor) {
      registerHook(target, 'beforeUpdate', descriptor.value);
    };
  }
}

export function AfterUpdate(target?: any, propertyKey?: string, descriptor?: PropertyDescriptor): any {
  if (target && propertyKey && descriptor) {
    registerHook(target, 'afterUpdate', descriptor.value);
  } else {
    return function(target: any, propertyKey: string, descriptor: PropertyDescriptor) {
      registerHook(target, 'afterUpdate', descriptor.value);
    };
  }
}

export function BeforeDelete(target?: any, propertyKey?: string, descriptor?: PropertyDescriptor): any {
  if (target && propertyKey && descriptor) {
    registerHook(target, 'beforeDelete', descriptor.value);
  } else {
    return function(target: any, propertyKey: string, descriptor: PropertyDescriptor) {
      registerHook(target, 'beforeDelete', descriptor.value);
    };
  }
}

export function AfterDelete(target?: any, propertyKey?: string, descriptor?: PropertyDescriptor): any {
  if (target && propertyKey && descriptor) {
    registerHook(target, 'afterDelete', descriptor.value);
  } else {
    return function(target: any, propertyKey: string, descriptor: PropertyDescriptor) {
      registerHook(target, 'afterDelete', descriptor.value);
    };
  }
}

export function BeforeSave(target: any, propertyKey: string, descriptor: PropertyDescriptor) {
  registerHook(target, 'beforeSave', descriptor.value);
}

export function AfterSave(target: any, propertyKey: string, descriptor: PropertyDescriptor) {
  registerHook(target, 'afterSave', descriptor.value);
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
  existingHooks.push(hookFunction.name);

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
