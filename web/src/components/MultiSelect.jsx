import { useState, useRef, useEffect } from 'react';
import { createPortal } from 'react-dom';
import { X, ChevronDown, Check } from 'lucide-react';

export default function MultiSelect({ 
  options = [], 
  value = [], 
  onChange, 
  placeholder = 'Select...',
  labelKey = 'label',
  valueKey = 'value',
  searchable = true
}) {
  const [isOpen, setIsOpen] = useState(false);
  const [search, setSearch] = useState('');
  const [dropdownStyle, setDropdownStyle] = useState({});
  const containerRef = useRef(null);
  const dropdownRef = useRef(null);
  const inputRef = useRef(null);

  useEffect(() => {
    const handleClickOutside = (e) => {
      const clickedContainer = containerRef.current?.contains(e.target);
      const clickedDropdown = dropdownRef.current?.contains(e.target);
      
      if (!clickedContainer && !clickedDropdown) {
        setIsOpen(false);
      }
    };
    
    if (isOpen) {
      document.addEventListener('mousedown', handleClickOutside);
      return () => document.removeEventListener('mousedown', handleClickOutside);
    }
  }, [isOpen]);

  useEffect(() => {
    if (isOpen && containerRef.current) {
      const rect = containerRef.current.getBoundingClientRect();
      const spaceBelow = window.innerHeight - rect.bottom;
      const spaceAbove = rect.top;
      const dropdownHeight = 250;
      
      const openAbove = spaceBelow < dropdownHeight && spaceAbove > spaceBelow;
      
      setDropdownStyle({
        position: 'fixed',
        left: rect.left,
        width: rect.width,
        ...(openAbove 
          ? { bottom: window.innerHeight - rect.top + 4 }
          : { top: rect.bottom + 4 }
        ),
        maxHeight: Math.min(dropdownHeight, openAbove ? spaceAbove - 20 : spaceBelow - 20)
      });
    }
  }, [isOpen]);

  const filteredOptions = options.filter(opt => {
    const label = typeof opt === 'string' ? opt : opt[labelKey];
    return label.toLowerCase().includes(search.toLowerCase());
  });

  const getLabel = (opt) => typeof opt === 'string' ? opt : opt[labelKey];
  const getValue = (opt) => typeof opt === 'string' ? opt : opt[valueKey];

  const isSelected = (opt) => value.includes(getValue(opt));

  const toggleOption = (opt) => {
    const val = getValue(opt);
    if (isSelected(opt)) {
      onChange(value.filter(v => v !== val));
    } else {
      onChange([...value, val]);
    }
  };

  const removeValue = (val, e) => {
    e.stopPropagation();
    onChange(value.filter(v => v !== val));
  };

  const getOptionByValue = (val) => {
    return options.find(opt => getValue(opt) === val);
  };

  return (
    <div ref={containerRef} className="relative">
      <div
        onClick={() => { setIsOpen(!isOpen); inputRef.current?.focus(); }}
        className="min-h-[42px] px-3 py-2 bg-zinc-900 border border-zinc-700 rounded-md cursor-pointer flex flex-wrap gap-1 items-center"
      >
        {value.length === 0 ? (
          <span className="text-zinc-500">{placeholder}</span>
        ) : (
          value.map(val => {
            const opt = getOptionByValue(val);
            const label = opt ? getLabel(opt) : val;
            return (
              <span
                key={val}
                className="inline-flex items-center gap-1 px-2 py-0.5 bg-zinc-700 text-zinc-200 text-sm rounded"
              >
                <span className="truncate max-w-[150px]">{label}</span>
                <button
                  type="button"
                  onClick={(e) => removeValue(val, e)}
                  className="text-zinc-400 hover:text-zinc-200"
                >
                  <X className="w-3 h-3" />
                </button>
              </span>
            );
          })
        )}
        <ChevronDown className={`w-4 h-4 text-zinc-400 ml-auto transition-transform ${isOpen ? 'rotate-180' : ''}`} />
      </div>

      {isOpen && createPortal(
        <div 
          ref={dropdownRef}
          style={dropdownStyle}
          className="z-[100] bg-zinc-800 border border-zinc-700 rounded-md shadow-lg overflow-hidden"
        >
          {searchable && (
            <div className="p-2 border-b border-zinc-700">
              <input
                ref={inputRef}
                type="text"
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                placeholder="Search..."
                className="w-full px-2 py-1 bg-zinc-900 border border-zinc-700 rounded text-zinc-100 text-sm focus:outline-none focus:border-blue-500"
              />
            </div>
          )}
          <div className="overflow-y-auto" style={{ maxHeight: dropdownStyle.maxHeight ? dropdownStyle.maxHeight - 50 : 200 }}>
            {filteredOptions.length === 0 ? (
              <div className="px-3 py-2 text-zinc-500 text-sm">No options found</div>
            ) : (
              filteredOptions.map(opt => {
                const selected = isSelected(opt);
                return (
                  <div
                    key={getValue(opt)}
                    onClick={() => toggleOption(opt)}
                    className={`px-3 py-2 cursor-pointer flex items-center gap-2 ${
                      selected ? 'bg-blue-500/20 text-zinc-100' : 'text-zinc-300 hover:bg-zinc-700'
                    }`}
                  >
                    <div className={`w-4 h-4 border rounded flex items-center justify-center ${
                      selected ? 'bg-blue-500 border-blue-500' : 'border-zinc-600'
                    }`}>
                      {selected && <Check className="w-3 h-3 text-white" />}
                    </div>
                    <span className="truncate">{getLabel(opt)}</span>
                  </div>
                );
              })
            )}
          </div>
        </div>,
        document.body
      )}
    </div>
  );
}
