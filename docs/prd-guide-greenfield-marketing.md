# PRD Writing Guide for Greenfield Marketing Sites

This guide explains how to write effective PRDs for new marketing websites built from scratch. Unlike feature PRDs for existing projects, greenfield PRDs must define architecture, tech stack, and project structure upfront since there's no existing codebase to reference.

## Key Principles

### 1. Define the Tech Stack Explicitly

Ralph cannot make technology choices. Specify frameworks, libraries, and tooling upfront.

**Bad:**

> Build a modern marketing site with good SEO.

**Good:**

> Build a Next.js 14 marketing site with App Router, Tailwind CSS, and TypeScript. Use `next/image` for optimized images. Deploy to Vercel with automatic preview deployments.

### 2. Specify Project Structure

Without an existing codebase, you must define where files should live.

**Bad:**

> Create components for the landing page.

**Good:**

> Create components following this structure:
>
> - `src/components/ui/` - Reusable primitives (Button, Card, Input)
> - `src/components/sections/` - Page sections (Hero, Features, Pricing)
> - `src/components/layout/` - Layout components (Header, Footer, Container)

### 3. Be Explicit About Design

Marketing sites are heavily design-driven. Provide specific design tokens, spacing, and visual requirements.

**Bad:**

> The hero section should look good.

**Good:**

> Hero section specifications:
>
> - Height: 100vh on desktop, auto on mobile
> - Background: gradient from `#1a1a2e` to `#16213e`
> - Headline: 64px/72px line-height, font-weight 700, color white
> - CTA button: 16px padding, 8px border-radius, `#4f46e5` background

### 4. Define Content Structure

Marketing sites need clear content hierarchy. Specify what content exists and where it comes from.

**Bad:**

> Add testimonials to the homepage.

**Good:**

> Testimonials section:
>
> - Display 3 testimonials in a grid (1 column mobile, 3 columns desktop)
> - Each testimonial: quote (max 150 chars), author name, author title, company, avatar
> - Content defined in `src/data/testimonials.ts` as typed array
> - No CMS integration for v1 (static data)

---

## PRD Structure for Greenfield Marketing Sites

### Required Sections

```markdown
# [Site Name] Marketing Site - PRD

## Overview

[What is this site? Who is it for? What's the primary conversion goal?]

## Goals

[Measurable outcomes: traffic, conversions, performance metrics]

## Non-Goals

[What's explicitly out of scope for v1]

## Tech Stack

[CRITICAL: All technology choices must be specified]

## Project Structure

[CRITICAL: Define file/folder organization]

## Design System

[Colors, typography, spacing, breakpoints]

## Pages & Routes

[All pages with their routes and purposes]

## Components

[Component hierarchy and specifications]

## Content Structure

[Where content comes from, data models]

## SEO Requirements

[Meta tags, structured data, sitemap]

## Performance Requirements

[Core Web Vitals targets, bundle size limits]

## Verification Commands

[How to verify the implementation]
```

### Recommended Sections

```markdown
## Analytics & Tracking

[GA4, conversion tracking, events]

## Forms & Integrations

[Contact forms, newsletter signup, CRM integration]

## Accessibility Requirements

[WCAG level, specific requirements]

## Deployment & Infrastructure

[Hosting, CDN, environment variables]

## Responsive Breakpoints

[Mobile, tablet, desktop specifications]

## Animation & Interactions

[Scroll animations, hover states, transitions]
```

---

## The Critical Section: Tech Stack

This section replaces "Existing Code Context" for greenfield projects. It defines the foundation.

### What to Include

```markdown
## Tech Stack

### Framework

- **Next.js 14** with App Router
- **React 18** with Server Components where appropriate
- **TypeScript** strict mode enabled

### Styling

- **Tailwind CSS 3.4** for utility-first styling
- **tailwind-merge** for conditional class merging
- **clsx** for className composition

### UI Components

- Build from scratch (no component library)
- Or: Use **shadcn/ui** as base, customize to match brand

### Images & Media

- **next/image** for optimization
- Images stored in `public/images/`
- SVG icons inline or via **lucide-react**

### Forms

- **react-hook-form** for form state
- **zod** for validation schemas

### Animation

- **framer-motion** for complex animations
- CSS transitions for simple hover/focus states

### Development

- **ESLint** with Next.js config
- **Prettier** for formatting
- **pnpm** as package manager

### Deployment

- **Vercel** for hosting
- Automatic preview deployments on PR
- Environment variables via Vercel dashboard
```

### Why This Matters

Without explicit tech stack definition, Ralph might:

- Choose incompatible library versions
- Mix different styling approaches
- Create inconsistent project structure
- Use deprecated patterns

---

## Project Structure Section

Define the complete folder structure before any implementation.

```markdown
## Project Structure
```

├── src/
│ ├── app/ # Next.js App Router
│ │ ├── layout.tsx # Root layout (fonts, metadata)
│ │ ├── page.tsx # Homepage
│ │ ├── about/
│ │ │ └── page.tsx
│ │ ├── pricing/
│ │ │ └── page.tsx
│ │ ├── contact/
│ │ │ └── page.tsx
│ │ └── globals.css # Tailwind imports + custom CSS
│ │
│ ├── components/
│ │ ├── ui/ # Primitives (Button, Card, Badge)
│ │ ├── sections/ # Page sections (Hero, Features)
│ │ ├── layout/ # Header, Footer, Container
│ │ └── forms/ # ContactForm, NewsletterForm
│ │
│ ├── lib/ # Utilities
│ │ ├── utils.ts # cn() helper, formatters
│ │ └── constants.ts # Site-wide constants
│ │
│ ├── data/ # Static content
│ │ ├── navigation.ts # Nav links
│ │ ├── features.ts # Feature list content
│ │ └── testimonials.ts # Testimonial content
│ │
│ └── types/ # TypeScript types
│ └── index.ts
│
├── public/
│ ├── images/ # Static images
│ ├── fonts/ # Custom fonts (if not using next/font)
│ └── favicon.ico
│
├── tailwind.config.ts
├── next.config.js
├── tsconfig.json
└── package.json

```

```

---

## Design System Section

Marketing sites require consistent visual design. Define tokens explicitly.

````markdown
## Design System

### Colors

```typescript
// tailwind.config.ts colors extension
colors: {
  brand: {
    50: '#f0f9ff',
    100: '#e0f2fe',
    500: '#0ea5e9',
    600: '#0284c7',
    700: '#0369a1',
  },
  gray: {
    50: '#f9fafb',
    100: '#f3f4f6',
    900: '#111827',
  },
}
```
````

### Typography

| Element | Size                | Weight | Line Height | Tracking |
| ------- | ------------------- | ------ | ----------- | -------- |
| h1      | 48px / 64px desktop | 700    | 1.1         | -0.02em  |
| h2      | 36px / 48px desktop | 600    | 1.2         | -0.01em  |
| h3      | 24px / 30px desktop | 600    | 1.3         | 0        |
| body    | 16px / 18px desktop | 400    | 1.6         | 0        |
| small   | 14px                | 400    | 1.5         | 0        |

Font: Inter via `next/font/google`

### Spacing Scale

Use Tailwind defaults with these common patterns:

- Section padding: `py-16 md:py-24`
- Container max-width: `max-w-7xl mx-auto px-4 sm:px-6 lg:px-8`
- Component gaps: `gap-4`, `gap-6`, `gap-8`

### Breakpoints

| Name | Min Width | Usage         |
| ---- | --------- | ------------- |
| sm   | 640px     | Large phones  |
| md   | 768px     | Tablets       |
| lg   | 1024px    | Small laptops |
| xl   | 1280px    | Desktops      |

### Border Radius

- Buttons: `rounded-lg` (8px)
- Cards: `rounded-xl` (12px)
- Badges: `rounded-full`
- Inputs: `rounded-md` (6px)

````

---

## Pages & Routes Section

List every page with its route, purpose, and key sections.

```markdown
## Pages & Routes

### Homepage (`/`)
**Purpose:** Primary landing page, drive signups
**Sections (in order):**
1. Hero - Headline, subhead, CTA, hero image
2. LogoCloud - Trusted by logos (6-8 companies)
3. Features - 3-column grid, icon + title + description
4. HowItWorks - 3-step process with illustrations
5. Testimonials - 3 customer quotes
6. Pricing - 3 tier cards (if not separate page)
7. CTA - Final conversion section
8. Footer

### About (`/about`)
**Purpose:** Company story, team, values
**Sections:**
1. Hero - Mission statement
2. Story - Company origin
3. Team - Team member grid
4. Values - Core values list

### Pricing (`/pricing`)
**Purpose:** Convert visitors to customers
**Sections:**
1. Hero - Pricing headline
2. PricingTable - 3 tiers with feature comparison
3. FAQ - Pricing-related questions
4. CTA - Contact sales

### Contact (`/contact`)
**Purpose:** Lead capture
**Sections:**
1. Hero - Contact headline
2. ContactForm - Name, email, company, message
3. ContactInfo - Email, phone, address
````

---

## Components Section

Specify components with their props and behavior.

````markdown
## Components

### UI Primitives (`src/components/ui/`)

#### Button

```typescript
interface ButtonProps {
  variant: "primary" | "secondary" | "outline" | "ghost";
  size: "sm" | "md" | "lg";
  children: React.ReactNode;
  href?: string; // Renders as Link if provided
  onClick?: () => void;
  disabled?: boolean;
  loading?: boolean;
}
```
````

- Primary: `bg-brand-600 hover:bg-brand-700 text-white`
- Secondary: `bg-gray-100 hover:bg-gray-200 text-gray-900`
- Loading state shows spinner, disables click

#### Card

```typescript
interface CardProps {
  children: React.ReactNode;
  className?: string;
  hover?: boolean; // Adds hover:shadow-lg transition
}
```

### Section Components (`src/components/sections/`)

#### Hero

```typescript
interface HeroProps {
  headline: string;
  subheadline: string;
  primaryCTA: { label: string; href: string };
  secondaryCTA?: { label: string; href: string };
  image?: { src: string; alt: string };
}
```

- Full viewport height on desktop
- Image on right (desktop), below text (mobile)
- Fade-in animation on load

#### Features

```typescript
interface Feature {
  icon: LucideIcon;
  title: string;
  description: string;
}

interface FeaturesProps {
  headline: string;
  subheadline?: string;
  features: Feature[]; // Exactly 3 or 6
}
```

- 3-column grid on desktop, 1-column mobile
- Icons rendered at 24x24, brand-600 color

````

---

## Content Structure Section

Define where content lives and its shape.

```markdown
## Content Structure

### Navigation (`src/data/navigation.ts`)
```typescript
export const mainNavigation = [
  { label: 'Features', href: '/#features' },
  { label: 'Pricing', href: '/pricing' },
  { label: 'About', href: '/about' },
  { label: 'Contact', href: '/contact' },
]

export const footerNavigation = {
  product: [
    { label: 'Features', href: '/#features' },
    { label: 'Pricing', href: '/pricing' },
  ],
  company: [
    { label: 'About', href: '/about' },
    { label: 'Contact', href: '/contact' },
  ],
  legal: [
    { label: 'Privacy', href: '/privacy' },
    { label: 'Terms', href: '/terms' },
  ],
}
````

### Features (`src/data/features.ts`)

```typescript
import { Zap, Shield, BarChart } from "lucide-react";

export const features = [
  {
    icon: Zap,
    title: "Lightning Fast",
    description: "Built for speed with optimized performance.",
  },
  // ... 2 more
];
```

### Testimonials (`src/data/testimonials.ts`)

```typescript
export const testimonials = [
  {
    quote: "This product transformed our workflow.",
    author: "Jane Smith",
    title: "CEO",
    company: "Acme Inc",
    avatar: "/images/testimonials/jane.jpg",
  },
  // ... 2 more
];
```

### Content Philosophy

- All content lives in `src/data/` as typed TypeScript
- No CMS for v1 (add later if needed)
- Images referenced by path in `public/images/`
- Content changes require code deployment

````

---

## SEO Requirements Section

Marketing sites live or die by SEO. Be specific.

```markdown
## SEO Requirements

### Metadata (per page)

Each page in `src/app/` exports metadata:

```typescript
// src/app/page.tsx
export const metadata: Metadata = {
  title: 'Product Name - Tagline',
  description: 'Primary value proposition in 150-160 characters.',
  openGraph: {
    title: 'Product Name - Tagline',
    description: 'Primary value proposition.',
    images: ['/images/og-home.jpg'],
  },
}
````

### Root Layout Metadata

```typescript
// src/app/layout.tsx
export const metadata: Metadata = {
  metadataBase: new URL("https://example.com"),
  title: {
    default: "Product Name",
    template: "%s | Product Name",
  },
  description: "Default site description.",
  robots: { index: true, follow: true },
};
```

### Structured Data

Add JSON-LD to homepage:

```typescript
// Organization schema
{
  "@context": "https://schema.org",
  "@type": "Organization",
  "name": "Company Name",
  "url": "https://example.com",
  "logo": "https://example.com/logo.png"
}
```

### Technical SEO

- Generate `sitemap.xml` via `next-sitemap`
- Generate `robots.txt` via `next-sitemap`
- All images have descriptive `alt` text
- Semantic HTML (`<main>`, `<article>`, `<section>`, `<nav>`)
- Heading hierarchy (one `<h1>` per page, sequential `<h2>`-`<h6>`)

````

---

## Performance Requirements Section

Marketing sites need fast Core Web Vitals for SEO and conversions.

```markdown
## Performance Requirements

### Core Web Vitals Targets

| Metric | Target | Measurement |
|--------|--------|-------------|
| LCP (Largest Contentful Paint) | < 2.5s | Hero image/headline |
| FID (First Input Delay) | < 100ms | Button clicks |
| CLS (Cumulative Layout Shift) | < 0.1 | No layout jumps |
| TTFB (Time to First Byte) | < 200ms | Server response |

### Implementation Requirements

1. **Images**
   - Use `next/image` for all images
   - Provide `width` and `height` to prevent CLS
   - Use `priority` for above-fold images
   - WebP format via automatic optimization

2. **Fonts**
   - Load via `next/font/google`
   - Use `display: swap` (automatic with next/font)
   - Subset to Latin characters only

3. **JavaScript**
   - No JavaScript for above-fold content rendering
   - Lazy load below-fold sections
   - Bundle size target: < 100KB first load JS

4. **CSS**
   - Tailwind purges unused styles automatically
   - No CSS-in-JS runtime

### Verification
```bash
# Run Lighthouse CI
npx lighthouse https://example.com --output=json

# Expected scores (local dev will vary):
# Performance: > 90
# Accessibility: > 90
# Best Practices: > 90
# SEO: > 90
````

````

---

## Verification Commands Section

```markdown
## Verification Commands

These commands must pass after implementation:

```bash
# Install dependencies
pnpm install

# Type checking
pnpm tsc --noEmit

# Linting
pnpm lint

# Build (catches build-time errors)
pnpm build

# Run dev server and verify manually
pnpm dev
````

### Manual Verification Checklist

#### Functionality

- [ ] All pages load without errors
- [ ] Navigation works (all links)
- [ ] Forms submit successfully
- [ ] Mobile menu opens/closes

#### Responsive Design

- [ ] Test at 375px (mobile)
- [ ] Test at 768px (tablet)
- [ ] Test at 1280px (desktop)
- [ ] No horizontal scroll at any width

#### Performance

- [ ] Lighthouse Performance > 90
- [ ] No CLS on page load
- [ ] Images lazy load below fold

#### SEO

- [ ] Each page has unique title/description
- [ ] OG images render in social preview
- [ ] Sitemap accessible at /sitemap.xml

````

---

## Common Mistakes in Greenfield PRDs

### 1. No Tech Stack Definition

**Bad:**
> Build a fast marketing website.

**Good:**
> Build with Next.js 14, App Router, TypeScript, Tailwind CSS. Deploy to Vercel. Use pnpm as package manager.

### 2. Vague Visual Requirements

**Bad:**
> The site should look modern and professional.

**Good:**
> Design system: Inter font, brand color #4f46e5, 8px border radius on buttons, 16px base spacing unit. Cards have subtle shadow (`shadow-sm`) and 1px gray-200 border.

### 3. Missing Content Specification

**Bad:**
> Add a features section.

**Good:**
> Features section displays exactly 3 features in a grid. Each feature has: Lucide icon (24px), title (h3, max 30 chars), description (max 100 chars). Content defined in `src/data/features.ts`.

### 4. Undefined Component Boundaries

**Bad:**
> Create reusable components.

**Good:**
> Component hierarchy:
> - `Button` - primitive, used everywhere
> - `Card` - primitive wrapper with shadow/border
> - `SectionHeading` - h2 + optional subhead
> - `Hero` - section, uses Button
> - `FeatureCard` - uses Card, icon, text

### 5. No Project Structure

**Bad:**
> Organize code well.

**Good:**
> Folder structure:
> - `src/app/` - routes (Next.js App Router)
> - `src/components/ui/` - primitives
> - `src/components/sections/` - page sections
> - `src/data/` - static content as TypeScript
> - `src/lib/` - utilities

---

## Template: Greenfield Marketing Site PRD

```markdown
# [Product Name] Marketing Site - PRD

## Overview

[2-3 sentences: What is this site? What's the conversion goal?]

## Goals

- Launch marketing site for [product]
- Achieve Lighthouse score > 90 on all metrics
- Mobile-responsive across all breakpoints
- SEO-optimized with proper meta tags and structured data

## Non-Goals

- Blog/content management system (future phase)
- User authentication
- Dynamic content from API
- Internationalization

## Tech Stack

### Core
- Next.js 14 (App Router)
- React 18
- TypeScript (strict mode)

### Styling
- Tailwind CSS 3.4
- tailwind-merge + clsx

### Development
- pnpm
- ESLint + Prettier
- Vercel deployment

## Project Structure

````

src/
├── app/
│ ├── layout.tsx
│ ├── page.tsx
│ ├── about/page.tsx
│ ├── pricing/page.tsx
│ └── contact/page.tsx
├── components/
│ ├── ui/
│ ├── sections/
│ └── layout/
├── data/
├── lib/
└── types/

````

## Design System

### Colors
- Brand: `#[hex]`
- Gray scale: Tailwind defaults
- Accent: `#[hex]`

### Typography
- Font: Inter via next/font
- Headings: 700 weight
- Body: 400 weight, 16px base

### Spacing
- Section padding: `py-16 md:py-24`
- Container: `max-w-7xl mx-auto px-4 sm:px-6 lg:px-8`

## Pages

### Homepage (`/`)
1. Hero
2. LogoCloud
3. Features
4. Testimonials
5. CTA
6. Footer

### [Additional pages...]

## Components

### UI Primitives
- Button (primary, secondary, outline)
- Card
- Badge
- Input

### Sections
- Hero
- Features
- Testimonials
- PricingTable
- CTA

### Layout
- Header
- Footer
- Container

## Content Structure

All content in `src/data/` as TypeScript:
- `navigation.ts` - nav links
- `features.ts` - feature list
- `testimonials.ts` - customer quotes
- `pricing.ts` - pricing tiers

## SEO Requirements

- Unique title/description per page
- Open Graph images (1200x630)
- JSON-LD Organization schema
- Sitemap and robots.txt via next-sitemap

## Performance Requirements

- LCP < 2.5s
- CLS < 0.1
- First load JS < 100KB
- Lighthouse Performance > 90

## Verification Commands

```bash
pnpm install
pnpm tsc --noEmit
pnpm lint
pnpm build
````

## Responsive Breakpoints

- Mobile: 375px
- Tablet: 768px
- Desktop: 1280px

## Accessibility

- WCAG 2.1 AA compliance
- Keyboard navigation
- Screen reader friendly
- Color contrast ratios

```

---

## Checklist Before Submitting Greenfield PRD to Ralph

- [ ] Tech stack fully specified (framework, styling, tooling)
- [ ] Project structure defined (folder organization)
- [ ] Design system documented (colors, typography, spacing)
- [ ] All pages listed with routes and sections
- [ ] Component hierarchy clear (primitives vs sections)
- [ ] Content structure defined (where data lives)
- [ ] SEO requirements explicit (meta, OG, structured data)
- [ ] Performance targets set (Core Web Vitals)
- [ ] Verification commands specified
- [ ] Non-goals clearly exclude future phases
- [ ] No ambiguous design decisions ("make it look good")
- [ ] Responsive breakpoints defined

---

This guide ensures your greenfield marketing site PRDs provide Ralph with everything needed to build a complete, production-ready marketing website from scratch.
```
