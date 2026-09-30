import { useEffect, useRef, useState } from "react";
import type { ChangeEvent, FormEvent } from "react";
import type { ProductInput } from "../../api/products";
import { uploadProductImage } from "../../api/products";
import type { ProductDetails, Product } from "../../types/product";
import type { Category } from "../../types/category";
import { getAdminCategories } from "../../api/categories";
import { ApiError } from "../../api/errors";
import { productKinds } from "../../catalog";
import ProductImage from "../../components/ProductImage";

type ProductFormProps = {
  initial?: Product;
  submitLabel: string;
  onSubmit: (input: ProductInput) => Promise<void>;
};

export default function ProductForm({
  initial,
  submitLabel,
  onSubmit,
}: ProductFormProps) {
  const [details, setDetails] = useState<ProductDetails>(
    initial?.details || {},
  );
  const [relatedInput, setRelatedInput] = useState(
    (initial?.details?.related_ids || []).join(","),
  );
  const [gallery, setGallery] = useState(
    (initial?.image_urls || []).join("\n"),
  );
  const [stockReason, setStockReason] = useState("");
  const [name, setName] = useState(initial?.name ?? "");
  const [slug, setSlug] = useState(initial?.slug ?? "");
  const [description, setDescription] = useState(initial?.description ?? "");
  const [price, setPrice] = useState(initial ? String(initial.price / 10) : "");
  const [stock, setStock] = useState(initial ? String(initial.stock) : "");
  const [imageUrl, setImageUrl] = useState(initial?.image_url ?? "");
  const [categoryId, setCategoryId] = useState(
    initial?.category_id != null ? String(initial.category_id) : "",
  );
  const [categories, setCategories] = useState<Category[]>([]);
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [uploadError, setUploadError] = useState("");
  const fileInputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    getAdminCategories()
      .then(setCategories)
      .catch(() => {
        /* Category dropdown is optional; leave it empty (uncategorized-only)
           if this fails rather than blocking the product form. */
      });
  }, []);

  const handleFileChange = async (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    // Reset the input value so selecting the same file again after an
    // error still fires onChange.
    event.target.value = "";
    if (!file) {
      return;
    }

    setUploadError("");
    setUploading(true);

    try {
      const result = await uploadProductImage(file);
      setImageUrl(result.url);
    } catch (err) {
      if (err instanceof ApiError) {
        if (err.status === 401) {
          setUploadError("نشست شما منقضی شده است. دوباره وارد شوید.");
        } else if (err.status === 413) {
          setUploadError("حجم فایل بیشتر از حد مجاز است (حداکثر ۵ مگابایت)");
        } else if (err.status === 400) {
          setUploadError(
            "فرمت فایل پشتیبانی نمی‌شود. فقط JPEG، PNG و WebP مجاز است",
          );
        } else {
          setUploadError(err.message);
        }
      } else {
        setUploadError(
          err instanceof Error
            ? err.message
            : "مشکلی در بارگذاری تصویر پیش آمد",
        );
      }
    } finally {
      setUploading(false);
    }
  };

  const handleClearImage = () => {
    setImageUrl("");
    setUploadError("");
    if (fileInputRef.current) {
      fileInputRef.current.value = "";
    }
  };

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setError("");

    const trimmedName = name.trim();
    const trimmedSlug = slug.trim();
    const priceNum = Number(price);
    const stockNum = Number(stock);

    if (!trimmedName || !trimmedSlug) {
      setError("نام و Slug الزامی است");
      return;
    }
    if (!Number.isFinite(priceNum) || priceNum < 0) {
      setError("قیمت باید عددی بزرگتر یا مساوی صفر باشد");
      return;
    }
    if (!Number.isInteger(stockNum) || stockNum < 0) {
      setError("موجودی باید عددی صحیح و بزرگتر یا مساوی صفر باشد");
      return;
    }

    if (initial && stockNum !== initial.stock && !stockReason.trim()) {
      setError("دلیل تغییر موجودی را وارد کنید.");
      return;
    }
    if (!details.kind) {
      setError("نوع محصول را انتخاب کنید.");
      return;
    }
    const ids = relatedInput.trim()
      ? relatedInput.split(/[,،]/).map((x) => Number(x.trim()))
      : [];
    if (
      ids.length > 8 ||
      ids.some((x) => !Number.isSafeInteger(x) || x < 1) ||
      new Set(ids).size !== ids.length
    ) {
      setError("شناسهٔ محصولات مکمل صحیح و بدون تکرار باشد.");
      return;
    }
    if (!Number.isSafeInteger(Math.round(priceNum * 10))) {
      setError("قیمت خارج از محدوده است.");
      return;
    }
    setSubmitting(true);

    try {
      await onSubmit({
        details: {
          ...details,
          related_ids: relatedInput.trim()
            ? relatedInput.split(/[,،]/).map((x) => Number(x.trim()))
            : [],
        },
        image_urls: gallery
          .split(/\r?\n/)
          .map((s) => s.trim())
          .filter(Boolean),
        stock_reason: stockReason,
        name: trimmedName,
        slug: trimmedSlug,
        description: description.trim(),
        price: Math.round(priceNum * 10),
        stock: stockNum,
        image_url: imageUrl.trim() || null,
        category_id: categoryId ? Number(categoryId) : null,
      });
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "مشکلی در ذخیره محصول پیش آمد",
      );
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <form onSubmit={handleSubmit}>
      <fieldset className="product-attributes">
        <legend>مشخصات محصول مراقبت از گیاه</legend>
        <label className="form-field">
          نوع محصول
          <select
            aria-label="نوع محصول"
            value={details.kind || ""}
            onChange={(e) => setDetails({ ...details, kind: e.target.value })}
          >
            <option value="">انتخاب کنید</option>
            {productKinds.map((k) => (
              <option key={k.value} value={k.value}>
                {k.label}
              </option>
            ))}
          </select>
        </label>
        {(
          [
            ["brand", "برند"],
            ["weight_volume", "وزن / حجم"],
            ["formulation", "شکل یا نوع مصرف"],
            ["suitable_for", "مناسب برای"],
            ["country", "کشور سازنده"],
            ["included", "اقلام داخل بسته"],
            ["article_slug", "Slug مقالهٔ راهنما"],
          ] as const
        ).map(([key, label]) => (
          <label className="form-field" key={key}>
            {label}
            <input
              value={details[key] || ""}
              maxLength={key === "article_slug" ? 255 : 160}
              onChange={(e) =>
                setDetails({ ...details, [key]: e.target.value })
              }
            />
          </label>
        ))}
        <label className="form-field">
          تعداد در بسته
          <input
            type="number"
            min={0}
            max={10000}
            value={details.pack_count || ""}
            onChange={(e) =>
              setDetails({ ...details, pack_count: Number(e.target.value) })
            }
          />
        </label>
        <label className="form-field">
          تاریخ انقضا
          <input
            type="date"
            value={details.expiry_date || ""}
            onChange={(e) =>
              setDetails({ ...details, expiry_date: e.target.value })
            }
          />
        </label>
        {(
          [
            ["composition", "ترکیبات"],
            ["usage", "روش مصرف"],
            ["benefits", "ویژگی‌ها و مزایا"],
            ["warnings", "هشدار مصرف"],
            ["delivery_info", "روش ارائهٔ آموزش / مشاوره"],
          ] as const
        ).map(([key, label]) => (
          <label className="form-field form-field--wide" key={key}>
            {label}
            <textarea
              value={details[key] || ""}
              maxLength={6000}
              rows={3}
              onChange={(e) =>
                setDetails({ ...details, [key]: e.target.value })
              }
            />
          </label>
        ))}
        <label className="form-field form-field--wide">
          شناسهٔ محصولات مکمل (با ویرگول، حداکثر ۸)
          <input
            value={relatedInput}
            onChange={(e) => setRelatedInput(e.target.value)}
            placeholder="مثلاً 12,15"
          />
          <small>
            شناسه‌ها در فهرست محصولات دیده می‌شوند. پیشنهادها از محصولات موجود
            انتخاب می‌شوند.
          </small>
        </label>
        <label className="form-field">
          گالری: هر آدرس HTTPS یا بارگذاری محلی در یک خط
          <textarea
            value={gallery}
            onChange={(e) => setGallery(e.target.value)}
          />
        </label>
        <label className="form-field">
          بارگذاری تصاویر گالری (حداکثر ۸ تصویر)
          <input
            type="file"
            multiple
            accept="image/jpeg,image/png,image/webp"
            disabled={uploading}
            onChange={async (e) => {
              const files = Array.from(e.target.files || []);
              e.target.value = "";
              const existing = gallery.split(/\r?\n/).filter(Boolean);
              if (existing.length + files.length > 8) {
                setError("حداکثر ۸ تصویر مجاز است.");
                return;
              }
              setUploading(true);
              try {
                const urls = [];
                for (const file of files) {
                  urls.push((await uploadProductImage(file)).url);
                }
                setGallery([...existing, ...urls].join("\n"));
              } catch (err) {
                setError(
                  err instanceof Error ? err.message : "بارگذاری انجام نشد.",
                );
              } finally {
                setUploading(false);
              }
            }}
          />
        </label>
        {initial && (
          <label className="form-field">
            دلیل تغییر موجودی
            <input
              value={stockReason}
              onChange={(e) => setStockReason(e.target.value)}
            />
          </label>
        )}
      </fieldset>
      <div className="form-field">
        <label htmlFor="product-name">نام</label>
        <input
          id="product-name"
          type="text"
          value={name}
          onChange={(event) => setName(event.target.value)}
          required
        />
      </div>

      <div className="form-field">
        <label htmlFor="product-slug">Slug</label>
        <input
          id="product-slug"
          type="text"
          value={slug}
          onChange={(event) => setSlug(event.target.value)}
          required
        />
      </div>

      <div className="form-field">
        <label htmlFor="product-description">توضیحات</label>
        <input
          id="product-description"
          type="text"
          value={description}
          onChange={(event) => setDescription(event.target.value)}
        />
      </div>

      <div className="form-field">
        <label htmlFor="product-price">قیمت (تومان)</label>
        <input
          id="product-price"
          type="number"
          min={0}
          value={price}
          onChange={(event) => setPrice(event.target.value)}
          required
        />
      </div>

      <div className="form-field">
        <label htmlFor="product-stock">موجودی</label>
        <input
          id="product-stock"
          type="number"
          min={0}
          value={stock}
          onChange={(event) => setStock(event.target.value)}
          required
        />
      </div>

      <div className="form-field">
        <label htmlFor="product-image-file">تصویر محصول</label>

        <div className="product-image-field__preview">
          <ProductImage
            src={imageUrl || null}
            alt={name || "پیش‌نمایش تصویر محصول"}
            className="product-image-field__preview-img"
            placeholderClassName="product-image-field__preview-placeholder"
          />
        </div>

        <input
          id="product-image-file"
          ref={fileInputRef}
          type="file"
          accept="image/jpeg,image/png,image/webp"
          onChange={handleFileChange}
          disabled={uploading || submitting}
        />

        {uploading && (
          <p className="state-message state-message--inline">
            در حال بارگذاری...
          </p>
        )}

        {uploadError && (
          <p className="alert alert-error" role="alert">
            {uploadError}
          </p>
        )}

        {imageUrl && !uploading && (
          <button
            type="button"
            className="btn btn-secondary btn-block"
            onClick={handleClearImage}
            disabled={submitting}
          >
            حذف تصویر / بدون تصویر
          </button>
        )}

        <p className="form-field__hint">
          فرمت‌های مجاز: JPEG، PNG، WebP — حداکثر ۵ مگابایت. می‌توانید به‌جای
          بارگذاری، آدرس تصویر خارجی را مستقیماً وارد کنید.
        </p>
        <input
          id="product-image-url"
          type="text"
          placeholder="یا آدرس تصویر را وارد کنید"
          value={imageUrl}
          onChange={(event) => setImageUrl(event.target.value)}
          disabled={uploading}
        />
      </div>

      <div className="form-field">
        <label htmlFor="product-category">دسته‌بندی</label>
        <select
          id="product-category"
          value={categoryId}
          onChange={(event) => setCategoryId(event.target.value)}
        >
          <option value="">بدون دسته‌بندی</option>
          {categories.map((category) => (
            <option key={category.id} value={category.id}>
              {category.name}
            </option>
          ))}
        </select>
      </div>

      {error && (
        <p className="alert alert-error" role="alert">
          {error}
        </p>
      )}

      <button
        type="submit"
        className="btn btn-primary btn-block"
        disabled={submitting || uploading}
      >
        {submitting ? "در حال ذخیره..." : submitLabel}
      </button>
    </form>
  );
}
