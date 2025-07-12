#!/usr/bin/env ts-node

/**
 * Documentation Test Runner
 * 
 * This script validates that all code examples in the documentation
 * are accurate and work with the current implementation.
 */

import * as fs from 'fs';
import * as path from 'path';
import { exec } from 'child_process';
import { promisify } from 'util';

const execAsync = promisify(exec);

interface CodeBlock {
  language: string;
  content: string;
  file: string;
  lineNumber: number;
}

interface TestResult {
  file: string;
  passed: number;
  failed: number;
  errors: string[];
}

interface ValidationError {
  type: 'syntax' | 'api' | 'import' | 'type';
  message: string;
  file: string;
  line?: number;
}

class DocumentationTestRunner {
  private docsPath: string;
  private results: TestResult[] = [];
  private validationErrors: ValidationError[] = [];

  constructor(docsPath: string = './docs') {
    this.docsPath = docsPath;
  }

  async run(): Promise<void> {
    console.log('🚀 Starting documentation validation...\n');

    try {
      // Find all markdown files
      const mdFiles = await this.findMarkdownFiles();
      console.log(`📄 Found ${mdFiles.length} documentation files\n`);

      // Extract and validate code blocks
      for (const file of mdFiles) {
        await this.validateFile(file);
      }

      // Generate report
      this.generateReport();

    } catch (error) {
      console.error('❌ Documentation validation failed:', error);
      process.exit(1);
    }
  }

  private async findMarkdownFiles(): Promise<string[]> {
    const files: string[] = [];
    
    const scanDirectory = (dir: string) => {
      const items = fs.readdirSync(dir);
      
      for (const item of items) {
        const fullPath = path.join(dir, item);
        const stat = fs.statSync(fullPath);
        
        if (stat.isDirectory()) {
          scanDirectory(fullPath);
        } else if (item.endsWith('.md') || item.endsWith('.mdx')) {
          files.push(fullPath);
        }
      }
    };

    scanDirectory(this.docsPath);
    return files;
  }

  private async validateFile(filePath: string): Promise<void> {
    console.log(`📝 Validating: ${path.relative(this.docsPath, filePath)}`);
    
    const content = fs.readFileSync(filePath, 'utf-8');
    const codeBlocks = this.extractCodeBlocks(content, filePath);
    
    const result: TestResult = {
      file: filePath,
      passed: 0,
      failed: 0,
      errors: []
    };

    for (const block of codeBlocks) {
      try {
        await this.validateCodeBlock(block);
        result.passed++;
        console.log(`  ✅ Code block at line ${block.lineNumber}`);
      } catch (error) {
        result.failed++;
        result.errors.push(`Line ${block.lineNumber}: ${error.message}`);
        console.log(`  ❌ Code block at line ${block.lineNumber}: ${error.message}`);
      }
    }

    this.results.push(result);
    console.log();
  }

  private extractCodeBlocks(content: string, filePath: string): CodeBlock[] {
    const blocks: CodeBlock[] = [];
    const lines = content.split('\n');
    
    let inCodeBlock = false;
    let currentBlock: string[] = [];
    let language = '';
    let startLine = 0;

    for (let i = 0; i < lines.length; i++) {
      const line = lines[i];
      
      if (line.startsWith('```')) {
        if (inCodeBlock) {
          // End of code block
          if (language === 'typescript' || language === 'ts' || language === 'javascript' || language === 'js') {
            blocks.push({
              language,
              content: currentBlock.join('\n'),
              file: filePath,
              lineNumber: startLine
            });
          }
          
          inCodeBlock = false;
          currentBlock = [];
        } else {
          // Start of code block
          language = line.slice(3).trim();
          startLine = i + 1;
          inCodeBlock = true;
        }
      } else if (inCodeBlock) {
        currentBlock.push(line);
      }
    }

    return blocks;
  }

  private async validateCodeBlock(block: CodeBlock): Promise<void> {
    // Skip non-executable blocks
    if (this.shouldSkipBlock(block.content)) {
      return;
    }

    // Check for syntax errors
    await this.checkSyntax(block);
    
    // Check for API consistency
    this.checkAPIConsistency(block);
    
    // Check imports
    this.checkImports(block);
    
    // Check types
    this.checkTypes(block);
  }

  private shouldSkipBlock(content: string): boolean {
    const skipPatterns = [
      /\/\/ Skip test/,
      /\/\* Skip test/,
      /interface\s+\w+/,
      /type\s+\w+\s*=/,
      /declare\s+/,
      /export\s+interface/,
      /export\s+type/,
      /^\s*\/\//,           // Comment-only blocks
      /^\s*\*\//,           // Comment blocks
      /Configuration/i,     // Configuration examples
      /\.\.\.$/m,           // Incomplete examples
    ];

    return skipPatterns.some(pattern => pattern.test(content));
  }

  private async checkSyntax(block: CodeBlock): Promise<void> {
    // Create temporary file
    const tempFile = path.join('/tmp', `doc-test-${Date.now()}.ts`);
    
    try {
      // Add necessary imports for framework code
      const fullCode = this.addNecessaryImports(block.content);
      fs.writeFileSync(tempFile, fullCode);
      
      // Check syntax with TypeScript compiler
      await execAsync(`npx tsc --noEmit --target es2020 --moduleResolution node ${tempFile}`);
      
    } catch (error) {
      // Clean up syntax error messages
      const cleanError = this.cleanCompilerError(error.message);
      throw new Error(`Syntax error: ${cleanError}`);
    } finally {
      // Clean up temp file
      if (fs.existsSync(tempFile)) {
        fs.unlinkSync(tempFile);
      }
    }
  }

  private addNecessaryImports(code: string): string {
    const imports = [
      "import { BaseModel, Model, Field, HasMany, BelongsTo, HasOne, ManyToMany } from '../../../src/framework/models/decorators';",
      "import { BeforeCreate, AfterCreate, BeforeUpdate, AfterUpdate, BeforeDelete, AfterDelete } from '../../../src/framework/models/decorators/hooks';",
      "import { DebrosFramework } from '../../../src/framework/DebrosFramework';",
      "import { QueryBuilder } from '../../../src/framework/query/QueryBuilder';",
      "",
      "// Mock types for documentation examples",
      "interface ValidationError extends Error { field: string; constraint: string; }",
      "interface DatabaseError extends Error { }",
      "interface ValidationResult { valid: boolean; errors: ValidationError[]; }",
      "interface PaginatedResult<T> { data: T[]; total: number; page: number; perPage: number; totalPages: number; hasNext: boolean; hasPrev: boolean; }",
      "",
      "// Mock functions for examples",
      "async function setupOrbitDB(): Promise<any> { return {}; }",
      "async function setupIPFS(): Promise<any> { return {}; }",
      "",
    ].join('\n');

    return imports + '\n' + code;
  }

  private cleanCompilerError(error: string): string {
    return error
      .replace(/\/tmp\/doc-test-\d+\.ts/g, 'example')
      .replace(/error TS\d+:/g, '')
      .split('\n')
      .filter(line => line.trim() && !line.includes('Found'))
      .slice(0, 3) // Take first few error lines
      .join(' ')
      .trim();
  }

  private checkAPIConsistency(block: CodeBlock): void {
    const problematicPatterns = [
      {
        pattern: /User\.where\(/,
        message: 'Use User.query().where() instead of static User.where()',
        fix: 'Replace with User.query().where()'
      },
      {
        pattern: /User\.orderBy\(/,
        message: 'Use User.query().orderBy() instead of static User.orderBy()',
        fix: 'Replace with User.query().orderBy()'
      },
      {
        pattern: /User\.limit\(/,
        message: 'Use User.query().limit() instead of static User.limit()',
        fix: 'Replace with User.query().limit()'
      },
      {
        pattern: /@Field\(\s*\{\s*type:\s*(String|Number|Boolean|Array|Object)/,
        message: 'Field types should be strings, not constructors',
        fix: 'Use @Field({ type: "string" }) instead of @Field({ type: String })'
      },
      {
        pattern: /getQueryExecutor\(\)/,
        message: 'getQueryExecutor() method does not exist in current implementation',
        fix: 'Remove or replace with available methods'
      }
    ];

    for (const { pattern, message, fix } of problematicPatterns) {
      if (pattern.test(block.content)) {
        this.validationErrors.push({
          type: 'api',
          message: `${message}. ${fix}`,
          file: block.file,
          line: block.lineNumber
        });
        throw new Error(message);
      }
    }
  }

  private checkImports(block: CodeBlock): void {
    const importLines = block.content
      .split('\n')
      .filter(line => line.trim().startsWith('import'));

    for (const importLine of importLines) {
      // Check for non-existent exports
      if (importLine.includes('from \'@debros/network\'')) {
        const invalidImports = [
          'QueryExecutor',
          'ValidationError',
          'DatabaseError',
          'PaginatedResult'
        ];

        for (const invalidImport of invalidImports) {
          if (importLine.includes(invalidImport)) {
            this.validationErrors.push({
              type: 'import',
              message: `${invalidImport} is not exported from @debros/network`,
              file: block.file,
              line: block.lineNumber
            });
            throw new Error(`Invalid import: ${invalidImport}`);
          }
        }
      }
    }
  }

  private checkTypes(block: CodeBlock): void {
    // Check for undefined types used in examples
    const undefinedTypes = [
      /: QueryPlan/,
      /: ComponentStatus/,
      /: MigrationContext/,
      /: SlowQuery/,
      /: QueryStats/
    ];

    for (const pattern of undefinedTypes) {
      if (pattern.test(block.content)) {
        const match = block.content.match(pattern);
        if (match) {
          this.validationErrors.push({
            type: 'type',
            message: `Type ${match[0].slice(2)} is not defined`,
            file: block.file,
            line: block.lineNumber
          });
          throw new Error(`Undefined type: ${match[0].slice(2)}`);
        }
      }
    }
  }

  private generateReport(): void {
    console.log('\n' + '='.repeat(60));
    console.log('📊 DOCUMENTATION VALIDATION REPORT');
    console.log('='.repeat(60));

    let totalPassed = 0;
    let totalFailed = 0;

    for (const result of this.results) {
      totalPassed += result.passed;
      totalFailed += result.failed;

      const status = result.failed === 0 ? '✅' : '❌';
      const filename = path.relative(this.docsPath, result.file);
      
      console.log(`${status} ${filename}: ${result.passed} passed, ${result.failed} failed`);
      
      if (result.errors.length > 0) {
        result.errors.forEach(error => {
          console.log(`   ❌ ${error}`);
        });
      }
    }

    console.log('\n' + '-'.repeat(60));
    console.log(`📈 SUMMARY: ${totalPassed} passed, ${totalFailed} failed`);
    
    if (this.validationErrors.length > 0) {
      console.log(`\n⚠️  ${this.validationErrors.length} validation issues found:`);
      
      const errorsByType = this.groupErrorsByType();
      for (const [type, errors] of Object.entries(errorsByType)) {
        console.log(`\n${type.toUpperCase()} ERRORS (${errors.length}):`);
        errors.forEach(error => {
          const filename = path.relative(this.docsPath, error.file);
          console.log(`  - ${filename}${error.line ? `:${error.line}` : ''}: ${error.message}`);
        });
      }
    }

    if (totalFailed > 0) {
      console.log('\n❌ Documentation validation failed!');
      console.log('Please fix the errors above before proceeding.');
      process.exit(1);
    } else {
      console.log('\n✅ All documentation examples are valid!');
    }
  }

  private groupErrorsByType(): Record<string, ValidationError[]> {
    const groups: Record<string, ValidationError[]> = {};
    
    for (const error of this.validationErrors) {
      if (!groups[error.type]) {
        groups[error.type] = [];
      }
      groups[error.type].push(error);
    }
    
    return groups;
  }
}

// CLI Interface
async function main() {
  const args = process.argv.slice(2);
  const docsPath = args[0] || './docs';
  
  console.log('🔍 DebrosFramework Documentation Test Runner');
  console.log(`📁 Documentation path: ${docsPath}\n`);
  
  if (!fs.existsSync(docsPath)) {
    console.error(`❌ Documentation path not found: ${docsPath}`);
    process.exit(1);
  }

  const runner = new DocumentationTestRunner(docsPath);
  await runner.run();
}

// Auto-fix script
class DocumentationAutoFixer {
  private fixes: Array<{ file: string; pattern: RegExp; replacement: string; description: string }> = [
    {
      file: '*',
      pattern: /User\.where\(/g,
      replacement: 'User.query().where(',
      description: 'Convert static where calls to query builder'
    },
    {
      file: '*',
      pattern: /User\.orderBy\(/g,
      replacement: 'User.query().orderBy(',
      description: 'Convert static orderBy calls to query builder'
    },
    {
      file: '*',
      pattern: /User\.limit\(/g,
      replacement: 'User.query().limit(',
      description: 'Convert static limit calls to query builder'
    },
    {
      file: '*',
      pattern: /@Field\(\s*\{\s*type:\s*String/g,
      replacement: '@Field({ type: \'string\'',
      description: 'Convert String type to string'
    },
    {
      file: '*',
      pattern: /@Field\(\s*\{\s*type:\s*Number/g,
      replacement: '@Field({ type: \'number\'',
      description: 'Convert Number type to number'
    },
    {
      file: '*',
      pattern: /@Field\(\s*\{\s*type:\s*Boolean/g,
      replacement: '@Field({ type: \'boolean\'',
      description: 'Convert Boolean type to boolean'
    },
    {
      file: '*',
      pattern: /@Field\(\s*\{\s*type:\s*Array/g,
      replacement: '@Field({ type: \'array\'',
      description: 'Convert Array type to array'
    },
    {
      file: '*',
      pattern: /@Field\(\s*\{\s*type:\s*Object/g,
      replacement: '@Field({ type: \'object\'',
      description: 'Convert Object type to object'
    }
  ];

  async fixDocumentation(docsPath: string): Promise<void> {
    console.log('🔧 Auto-fixing documentation issues...\n');
    
    const mdFiles = await this.findMarkdownFiles(docsPath);
    let totalFixes = 0;

    for (const file of mdFiles) {
      const fixes = await this.fixFile(file);
      totalFixes += fixes;
    }

    console.log(`\n✅ Applied ${totalFixes} automatic fixes`);
  }

  private async findMarkdownFiles(docsPath: string): Promise<string[]> {
    const files: string[] = [];
    
    const scanDirectory = (dir: string) => {
      const items = fs.readdirSync(dir);
      
      for (const item of items) {
        const fullPath = path.join(dir, item);
        const stat = fs.statSync(fullPath);
        
        if (stat.isDirectory()) {
          scanDirectory(fullPath);
        } else if (item.endsWith('.md') || item.endsWith('.mdx')) {
          files.push(fullPath);
        }
      }
    };

    scanDirectory(docsPath);
    return files;
  }

  private async fixFile(filePath: string): Promise<number> {
    let content = fs.readFileSync(filePath, 'utf-8');
    let fixes = 0;
    
    for (const fix of this.fixes) {
      const matches = content.match(fix.pattern);
      if (matches) {
        content = content.replace(fix.pattern, fix.replacement);
        fixes += matches.length;
        console.log(`  ✅ ${path.relative('./docs', filePath)}: ${fix.description} (${matches.length} fixes)`);
      }
    }
    
    if (fixes > 0) {
      fs.writeFileSync(filePath, content);
    }
    
    return fixes;
  }
}

// Add CLI command for auto-fix
if (process.argv.includes('--fix')) {
  const docsPath = process.argv[process.argv.indexOf('--fix') + 1] || './docs';
  const fixer = new DocumentationAutoFixer();
  fixer.fixDocumentation(docsPath).catch(console.error);
} else {
  main().catch(console.error);
}

export { DocumentationTestRunner, DocumentationAutoFixer };
