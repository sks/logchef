import type { ASTNode } from './types';
import type { SchemaInfo } from './sql-generator';

interface CacheEntry {
  ast: ASTNode;
  sql: string;
  params: unknown[];
  timestamp: number;
  accessCount: number;
}

export class QueryCache {
  private cache = new Map<string, CacheEntry>();
  private maxSize: number;
  private ttl: number;

  constructor(maxSize: number = 100, ttl: number = 5 * 60 * 1000) {
    this.maxSize = maxSize;
    this.ttl = ttl;
  }

  get(query: string, options?: { schema?: SchemaInfo }): {
    ast: ASTNode;
    sql: string;
    params: unknown[];
  } | null {
    const key = this.generateCacheKey(query, options);
    const entry = this.cache.get(key);

    if (!entry) return null;

    // Check TTL
    if (Date.now() - entry.timestamp > this.ttl) {
      this.cache.delete(key);
      return null;
    }

    // Update access statistics
    entry.accessCount++;
    return {
      ast: entry.ast,
      sql: entry.sql,
      params: entry.params
    };
  }

  set(
    query: string,
    ast: ASTNode,
    sql: string,
    params: unknown[],
    options?: { schema?: SchemaInfo }
  ): void {
    const key = this.generateCacheKey(query, options);

    // Implement LRU eviction if cache is full
    if (this.cache.size >= this.maxSize) {
      this.evictLeastRecentlyUsed();
    }

    this.cache.set(key, {
      ast,
      sql,
      params,
      timestamp: Date.now(),
      accessCount: 1
    });
  }

  private generateCacheKey(query: string, options?: { schema?: SchemaInfo }): string {
    const schemaHash = options?.schema
      ? this.hash(JSON.stringify(options.schema))
      : 'no-schema';
    return `${query}:${schemaHash}`;
  }

  private hash(str: string): string {
    // Simple hash function for cache keys
    let hash = 0;
    for (let i = 0; i < str.length; i++) {
      const char = str.charCodeAt(i);
      hash = ((hash << 5) - hash) + char;
      hash = hash & hash; // Convert to 32-bit integer
    }
    return hash.toString();
  }

  private evictLeastRecentlyUsed(): void {
    let oldestKey: string | null = null;
    let oldestTime = Date.now();

    for (const [key, entry] of this.cache.entries()) {
      if (entry.timestamp < oldestTime) {
        oldestTime = entry.timestamp;
        oldestKey = key;
      }
    }

    if (oldestKey) {
      this.cache.delete(oldestKey);
    }
  }

  clear(): void {
    this.cache.clear();
  }

  getStats(): { size: number; hitRate: number; totalAccesses: number } {
    const entries = Array.from(this.cache.values());
    const totalAccesses = entries.reduce((sum, entry) => sum + entry.accessCount, 0);
    const totalRequests = totalAccesses; // This would need to be tracked separately

    return {
      size: this.cache.size,
      hitRate: totalAccesses > 0 ? (totalAccesses / totalRequests) * 100 : 0,
      totalAccesses
    };
  }
}