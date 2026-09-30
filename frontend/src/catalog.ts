export const productKinds = [
  {
    value: "fertilizer",
    slug: "fertilizers",
    label: "کود و تقویت‌کننده‌ها",
    description: "انتخاب کود و مکمل با توجه به ترکیبات، نوع مصرف و نیاز گیاه.",
    icon: "leaf",
  },
  {
    value: "substrate",
    slug: "substrates",
    label: "خاک و بستر کشت",
    description: "خاک آماده، پرلیت، کوکوپیت و ترکیبات بستر برای گلدان شما.",
    icon: "grid",
  },
  {
    value: "tool",
    slug: "tools",
    label: "ابزار و ملزومات",
    description: "ابزار هرس، آبیاری و رسیدگی روزمره به گیاهان.",
    icon: "drop",
  },
  {
    value: "protection",
    slug: "protection",
    label: "مراقبت و محافظت",
    description: "محصولات محافظتی همراه با روش مصرف و هشدارهای سازنده.",
    icon: "shield",
  },
  {
    value: "education",
    slug: "education",
    label: "آموزش و مشاوره",
    description:
      "آموزش، فایل و مشاوره؛ روش ارائهٔ هر خدمت در صفحهٔ آن مشخص می‌شود.",
    icon: "user",
  },
  {
    value: "bundle",
    slug: "bundles",
    label: "پک‌های مراقبت",
    description:
      "پک‌های آماده با اقلام مشخص؛ هر پک یک محصول با موجودی مستقل است.",
    icon: "bag",
  },
] as const;
export function kindLabel(value?: string) {
  return (
    productKinds.find((k) => k.value === value)?.label || "محصول مراقبت از گیاه"
  );
}
