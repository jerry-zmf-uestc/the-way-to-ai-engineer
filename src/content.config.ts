import { defineCollection } from 'astro:content';
import { glob } from 'astro/loaders';
import { z } from 'astro/zod';

const chapters = defineCollection({
  loader: glob({ pattern: '**/*.md', base: './src/content/chapters' }),
  schema: z.object({
    pageId: z.string(),
    title: z.string(),
    chapter: z.number().int().positive().optional(),
    part: z.string(),
    unitNumber: z.number().int().min(1).max(37).optional(),
    navigationPath: z.array(z.string()).length(1).optional(),
    order: z.number().int().nonnegative(),
    description: z.string(),
    published: z.boolean(),
    source: z.string(),
    labs: z.array(z.string()).default([]),
    capabilities: z.array(z.string()).default([]),
  }),
});

export const collections = { chapters };
